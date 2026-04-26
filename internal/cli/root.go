package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
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
	exitOK      = 0
	exitNo      = 1  // decision mode only
	exitUnknown = 2  // decision mode only
	exitRuntime = 10 // network, API
	exitUsage   = 11 // bad flags, bad config
	exitSigint  = 130
)

// rootFlags carries the parsed CLI flag values.
type rootFlags struct {
	profile     string
	model       string
	ifMode      bool
	unlessMode  bool
	interactive bool
	incognito   bool
	maxInput    int64
	timeout     time.Duration
	configure   bool
	showVersion bool
	stats       bool
}

// Version is set via -ldflags by goreleaser. When unset (e.g. `go install
// ...@vX.Y.Z` or a local `go build`), resolveVersion falls back to the
// module version embedded by the Go toolchain.
var Version = "dev"

func resolveVersion() string {
	if Version != "dev" {
		return Version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Version
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	return Version
}

// Execute is the entrypoint called from main. Returns the desired exit
// code. It never panics on user-triggered errors.
func Execute() int {
	var flags rootFlags

	cmd := &cobra.Command{
		Use:   "qq [QUESTION...]",
		Short: "A single-shot LLM you can pipe into, script with, and branch on",
		// We handle --help ourselves so usage output doesn't print on errors.
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.showVersion {
				fmt.Fprintln(cmd.OutOrStdout(), resolveVersion())
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
	cmd.Flags().BoolVarP(&flags.interactive, "interactive", "i", false, "preview response on /dev/tty and confirm before writing to stdout")
	cmd.Flags().BoolVar(&flags.incognito, "incognito", false, "skip history for this invocation")
	cmd.Flags().Int64Var(&flags.maxInput, "max-input", 0, "cap stdin bytes (default 200KiB)")
	cmd.Flags().DurationVar(&flags.timeout, "timeout", 0, "per-request timeout (default 120s, overrides profile and config)")
	cmd.Flags().BoolVar(&flags.configure, "configure", false, "interactive setup (adds/edits profiles)")
	cmd.Flags().BoolVar(&flags.showVersion, "version", false, "print version and exit")
	cmd.Flags().BoolVar(&flags.stats, "stats", false, "print token usage and timing to stderr after the response")

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
	if flags.interactive && (flags.ifMode || flags.unlessMode) {
		return usageErrorf("--interactive cannot be combined with --if/--unless")
	}
	if flags.interactive && term.IsTerminal(int(os.Stdout.Fd())) {
		return usageErrorf("--interactive requires stdout to be piped or redirected")
	}
	if flags.timeout < 0 {
		return usageErrorf("--timeout must be positive, got %s", flags.timeout)
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
		arg = strings.Join(args, " ")
	}

	stdinReader, stdinTTY := stdin, func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }
	if !argGiven && stdinTTY() {
		return usageErrorf("no input: pass a question as an argument or pipe content to stdin")
	}

	maxInput := int(flags.maxInput)
	if maxInput <= 0 {
		maxInput = cfg.InputMaxBytes()
	}

	in, err := input.Resolve(input.Options{
		Arg:             arg,
		ArgGiven:        argGiven,
		MaxInput:        maxInput,
		Stdin:           stdinReader,
		StdinIsTerminal: stdinTTY,
	})
	if err != nil {
		return usageErrorf("%s", err)
	}
	if in.Truncated {
		if cfg.InputOnOverflow() == config.OnOverflowError {
			return usageErrorf("stdin exceeds %d bytes; raise --max-input or set input.on_overflow = %q", maxOrDefault(maxInput), config.OnOverflowTruncate)
		}
		fmt.Fprintf(stderr, "qq: stdin truncated at %d bytes; use --max-input to override\n", maxOrDefault(maxInput))
	}

	decisionMode := flags.ifMode || flags.unlessMode
	sysPrompt := client.ComposeSystemPrompt(resolved.SystemPrompt, in.ContentTag, decisionMode)

	ctx, cancel := context.WithTimeout(parent, resolveTimeout(flags.timeout, resolved.Timeout, cfg))
	defer cancel()

	spin := startSpinner(stderr)
	defer spin.stop()

	// In decision mode, stdout is reserved for passing stdin through on a
	// gate-open verdict, so the model's prose goes to stderr. In normal
	// mode, the prose IS the output and goes to stdout as before. In
	// interactive mode, the response streams live to /dev/tty for the
	// human to read, and is buffered for stdout — only flushed there if
	// they accept the prompt.
	modelOut := io.Writer(stdout)
	var pipeBuf bytes.Buffer
	var ttyFile *os.File
	var ttyReader *bufio.Reader
	onFirstWrite := spin.clear
	if decisionMode {
		modelOut = stderr
	} else if flags.interactive {
		f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
		if err != nil {
			return usageErrorf("--interactive requires /dev/tty: %s", err)
		}
		defer f.Close()
		// Switch to the alt screen only when the first byte arrives, so
		// the spinner runs on the normal screen and the user sees a
		// status before the modal preview appears. The flag tracks
		// whether we entered, so the deferred restore is a no-op on an
		// early return (e.g. an HTTP error before any content).
		var altEntered bool
		defer func() {
			if altEntered {
				fmt.Fprint(f, "\x1b[?1049l")
			}
		}()
		ttyFile = f
		ttyReader = bufio.NewReader(f)
		modelOut = io.MultiWriter(ttyFile, &pipeBuf)
		onFirstWrite = func() {
			spin.clear()
			fmt.Fprint(f, "\x1b[?1049h\x1b[H")
			altEntered = true
		}
	}

	// Start the stream; onFirstWrite fires on the first content byte —
	// clears the spinner, and (in interactive mode) switches to the alt
	// screen so the preview is modal.
	start := time.Now()
	resp, runErr := client.Run(ctx, client.Request{
		BaseURL:      resolved.BaseURL,
		APIKey:       resolved.APIKey,
		Model:        resolved.Model,
		SystemPrompt: sysPrompt,
		UserMessage:  in.UserMessage,
		Decision:     decisionMode,
	}, newFirstWriteTap(modelOut, onFirstWrite), stderr)
	elapsed := time.Since(start)

	spin.stop()

	if flags.stats && resp != nil && runErr == nil {
		fmt.Fprintln(stderr, formatStats(resp, resolved.Model, elapsed))
	}

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
		if resp.UsageKnown {
			entry.PromptTokens = resp.Usage.PromptTokens
			entry.CompletionTokens = resp.Usage.CompletionTokens
			entry.TotalTokens = resp.Usage.TotalTokens
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
		if flags.interactive {
			return confirmAndPipe(ttyFile, ttyReader, &pipeBuf, stdout)
		}
		return nil
	}

	exitErr := decisionExitError(resp.Decision, flags.ifMode)
	// Gate open (exit 0) → pass stdin through so `cmd | qq --unless "..." |
	// next` works like a `grep`-style filter. On no/unknown, stdout stays
	// empty; the verdict is carried by the exit code. When stdin was
	// truncated, the passthrough carries the truncated prefix — the
	// warning was already printed on stderr upstream.
	if exitErr == nil && in.StdinContent != "" {
		if _, err := io.WriteString(stdout, in.StdinContent); err != nil {
			return runtimeErrorf("write passthrough: %s", err)
		}
	}
	return exitErr
}

// confirmAndPipe prompts the user on /dev/tty and, on accept, flushes the
// buffered response to stdout. Default is no — only an explicit "y"/"yes"
// (any case) accepts; anything else, including empty input, aborts. On
// abort the buffered response is dropped and the caller exits 130 — same
// class as Ctrl-C, since the user explicitly stopped the pipeline.
func confirmAndPipe(tty io.Writer, reader *bufio.Reader, buf *bytes.Buffer, stdout io.Writer) error {
	fmt.Fprint(tty, "\nPipe to next command? [y/N] ")
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return runtimeErrorf("read confirmation: %s", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	accepted := answer == "y" || answer == "yes"
	if !accepted {
		return &cliError{code: exitSigint, err: errors.New("aborted")}
	}
	if _, err := io.Copy(stdout, buf); err != nil {
		return runtimeErrorf("write piped output: %s", err)
	}
	return nil
}

func maxOrDefault(n int) int {
	if n <= 0 {
		return input.DefaultMaxInput
	}
	return n
}

// resolveTimeout picks the per-request timeout: --timeout flag, then the
// profile's `timeout`, then config.toml's `request.timeout`, then the
// built-in default. Each layer is skipped when its value is zero/unset.
func resolveTimeout(flag, profile time.Duration, cfg *config.Config) time.Duration {
	if flag > 0 {
		return flag
	}
	if profile > 0 {
		return profile
	}
	return cfg.RequestTimeout()
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

// formatStats renders the --stats line. `cached=` and `finish=` are
// omitted rather than printed with empty/zero values, so the line stays
// terse for providers that don't report them.
func formatStats(resp *client.Response, model string, elapsed time.Duration) string {
	var b strings.Builder
	b.WriteString("qq: stats: ")
	if resp.UsageKnown {
		fmt.Fprintf(&b, "tokens=%d/%d (%d total",
			resp.Usage.PromptTokens, resp.Usage.CompletionTokens, resp.Usage.TotalTokens)
		if resp.Usage.CachedTokens > 0 {
			fmt.Fprintf(&b, ", %d cached", resp.Usage.CachedTokens)
		}
		b.WriteString(")")
	} else {
		b.WriteString("tokens=unknown")
	}
	fmt.Fprintf(&b, " latency=%.2fs model=%s", elapsed.Seconds(), model)
	if resp.FinishReason != "" {
		fmt.Fprintf(&b, " finish=%s", resp.FinishReason)
	}
	return b.String()
}
