package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/mscansian/qq/internal/client"
	"github.com/mscansian/qq/internal/config"
	"github.com/mscansian/qq/internal/history"
	"github.com/mscansian/qq/internal/input"
)

// Exit codes — universal, per ENGINEERING.md §Error handling.
const (
	exitOK       = 0
	exitNo       = 1  // decision mode only
	exitUnknown  = 2  // decision mode only
	exitRuntime  = 10 // network, API
	exitUsage    = 11 // bad flags, bad config
	exitSigint   = 130
	requestTimeo = 120 * time.Second
)

// rootFlags carries the parsed CLI flag values.
type rootFlags struct {
	profile     string
	model       string
	ifMode      bool
	unlessMode  bool
	incognito   bool
	maxInput    int64
	configure   bool
	showVersion bool
}

// Version is set via -ldflags at build time.
var Version = "dev"

// Execute is the entrypoint called from main. Returns the desired exit
// code. It never panics on user-triggered errors.
func Execute() int {
	var flags rootFlags

	cmd := &cobra.Command{
		Use:   "qq [QUESTION]",
		Short: "Quick Question — a terminal assistant for quick questions",
		// We handle --help ourselves so usage output doesn't print on errors.
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.showVersion {
				fmt.Fprintln(cmd.OutOrStdout(), Version)
				return nil
			}
			if flags.configure {
				return runConfigure(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
			}
			return runAsk(cmd.Context(), &flags, args, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}

	cmd.Flags().StringVarP(&flags.profile, "profile", "p", "", "use profile NAME")
	cmd.Flags().StringVarP(&flags.model, "model", "m", "", "override model for this invocation")
	cmd.Flags().BoolVar(&flags.ifMode, "if", false, "decision mode: exit 0 on yes, 1 on no, 2 on unknown")
	cmd.Flags().BoolVar(&flags.unlessMode, "unless", false, "inverted decision mode: exit 0 on no, 1 on yes, 2 on unknown")
	cmd.Flags().BoolVar(&flags.incognito, "incognito", false, "skip history for this invocation")
	cmd.Flags().Int64Var(&flags.maxInput, "max-input", 0, "cap stdin bytes (default 200KiB)")
	cmd.Flags().BoolVar(&flags.configure, "configure", false, "interactive setup (adds/edits profiles)")
	cmd.Flags().BoolVar(&flags.showVersion, "version", false, "print version and exit")

	// Wire SIGINT → cancel the request context.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	err := cmd.ExecuteContext(ctx)
	if err == nil {
		return exitOK
	}
	return mapExitError(err, cmd.ErrOrStderr())
}

// cliError lets runAsk / runConfigure return an error that already carries
// the intended exit code.
type cliError struct {
	code int
	err  error
}

func (e *cliError) Error() string { return e.err.Error() }
func (e *cliError) Unwrap() error { return e.err }

func usageErrorf(format string, a ...any) error {
	return &cliError{code: exitUsage, err: fmt.Errorf(format, a...)}
}

func runtimeErrorf(format string, a ...any) error {
	return &cliError{code: exitRuntime, err: fmt.Errorf(format, a...)}
}

// runAsk is the normal "ask a question" flow.
func runAsk(parent context.Context, flags *rootFlags, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if flags.ifMode && flags.unlessMode {
		return usageErrorf("--if and --unless are mutually exclusive")
	}

	// Load config first so we can fail fast on bad TOML.
	creds, err := config.LoadCredentials()
	if err != nil {
		return usageErrorf("%s", err)
	}
	if creds.ModeWarning != "" {
		fmt.Fprintln(stderr, creds.ModeWarning)
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		return usageErrorf("%s", err)
	}

	resolved, err := config.Resolve(creds, config.Overrides{
		Profile: flags.profile,
		Model:   flags.model,
	})
	if err != nil {
		return usageErrorf("%s", err)
	}

	// Zero-arg + stdin-is-TTY → print help.
	arg := ""
	argGiven := len(args) > 0
	if argGiven {
		arg = args[0]
	}

	stdinReader, stdinTTY := stdin, func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }
	if !argGiven && stdinTTY() {
		return usageErrorf("no input: pass a question as an argument or pipe content to stdin")
	}

	in, err := input.Resolve(input.Options{
		Arg:             arg,
		ArgGiven:        argGiven,
		MaxInput:        int(flags.maxInput),
		Stdin:           stdinReader,
		StdinIsTerminal: stdinTTY,
	})
	if err != nil {
		return usageErrorf("%s", err)
	}
	if in.Truncated {
		fmt.Fprintf(stderr, "qq: stdin truncated at %d bytes; use --max-input to override\n", maxOrDefault(int(flags.maxInput)))
	}

	decisionMode := flags.ifMode || flags.unlessMode
	sysPrompt := client.ComposeSystemPrompt(resolved.SystemPrompt, decisionMode)

	ctx, cancel := context.WithTimeout(parent, requestTimeo)
	defer cancel()

	spin := startSpinner(stderr)
	defer spin.stop()

	// Start the stream; the spinner clears on first write.
	resp, runErr := client.Run(ctx, client.Request{
		BaseURL:      resolved.BaseURL,
		APIKey:       resolved.APIKey,
		Model:        resolved.Model,
		SystemPrompt: sysPrompt,
		UserMessage:  in.UserMessage,
		Decision:     decisionMode,
	}, newFirstWriteTap(stdout, spin.clear), stderr)

	spin.stop()

	// Record history regardless of error — matches what was printed.
	skipHistory := flags.incognito || resolved.Incognito || !cfg.HistoryEnabled()
	if !skipHistory && resp != nil {
		entry := history.Entry{
			Timestamp: time.Now().UTC(),
			Profile:   resolved.ProfileName,
			Model:     resolved.Model,
			Question:  in.Question,
			Answer:    resp.Prose,
		}
		if decisionMode {
			entry.Decision = string(resp.Decision)
		}
		if herr := history.Append(entry, cfg.HistoryMaxEntries()); herr != nil {
			fmt.Fprintf(stderr, "qq: warning: failed to write history: %s\n", herr)
		}
	}

	if runErr != nil {
		if errors.Is(runErr, context.Canceled) {
			fmt.Fprintln(stderr)
			return &cliError{code: exitSigint, err: runErr}
		}
		return runtimeErrorf("%s", runErr)
	}

	if !decisionMode {
		return nil
	}
	return decisionExitError(resp.Decision, flags.ifMode)
}

func maxOrDefault(n int) int {
	if n <= 0 {
		return input.DefaultMaxInput
	}
	return n
}

// decisionExitError maps a verdict + mode to the right exit code.
func decisionExitError(d client.Decision, ifMode bool) error {
	switch d {
	case client.DecisionYes:
		if ifMode {
			return nil // exit 0
		}
		return &cliError{code: exitNo, err: errors.New("")}
	case client.DecisionNo:
		if ifMode {
			return &cliError{code: exitNo, err: errors.New("")}
		}
		return nil
	default:
		return &cliError{code: exitUnknown, err: errors.New("")}
	}
}

// mapExitError renders the error to stderr (when it has a message) and
// returns the exit code.
func mapExitError(err error, stderr io.Writer) int {
	var ce *cliError
	if errors.As(err, &ce) {
		if ce.err != nil && ce.err.Error() != "" {
			fmt.Fprintf(stderr, "qq: %s\n", ce.err)
		}
		return ce.code
	}
	// Cobra flag-parsing errors land here.
	fmt.Fprintf(stderr, "qq: %s\n", err)
	return exitUsage
}

// firstWriteTap wraps an io.Writer so a one-shot side effect runs on the
// first non-empty write — used to clear the spinner as soon as the first
// token arrives.
type firstWriteTap struct {
	w     io.Writer
	first *sync.Once
	fn    func()
}

func newFirstWriteTap(w io.Writer, fn func()) *firstWriteTap {
	return &firstWriteTap{w: w, first: &sync.Once{}, fn: fn}
}

func (t *firstWriteTap) Write(b []byte) (int, error) {
	if len(b) > 0 && t.fn != nil {
		t.first.Do(t.fn)
	}
	return t.w.Write(b)
}
