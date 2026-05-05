package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/mscansian/qq/internal/history"
)

// runLast prints the most recent history entry's question + answer in a
// prompt-friendly form, then exits. No model call, no history append.
//
// The intended use is shell continuation:
//
//	qq "$(qq --last)
//
//	but what about X?"
//
// Pre-feature entries lack PayloadBytes; we treat zero as "no extra
// payload beyond Question" and print no marker, matching the behavior
// for arg-only and stdin-only invocations recorded after the field was
// added.
func runLast(flags *rootFlags, args []string, _ io.Reader, stdout io.Writer) error {
	if err := rejectLastConflicts(flags, args); err != nil {
		return err
	}

	entry, err := history.Last()
	if errors.Is(err, history.ErrNoHistory) {
		return usageErrorf("no history yet; --last needs a previous non-incognito invocation")
	}
	if err != nil {
		return runtimeErrorf("read history: %s", err)
	}

	fmt.Fprintf(stdout, "Previous question: %s\n", entry.Question)
	if entry.PayloadBytes > 0 {
		fmt.Fprintf(stdout, "(previous turn included ~%s of piped content, not shown)\n", humanBytes(entry.PayloadBytes))
	}
	fmt.Fprintf(stdout, "Previous answer: %s\n", entry.Answer)
	return nil
}

// rejectLastConflicts errors if --last is combined with anything that
// only makes sense with a model call. Each branch names the offending
// flag in the message so the user knows what to drop.
func rejectLastConflicts(flags *rootFlags, args []string) error {
	if len(args) > 0 {
		return usageErrorf("--last takes no question argument")
	}
	switch {
	case flags.profile != "":
		return usageErrorf("--last cannot be combined with --profile")
	case flags.model != "":
		return usageErrorf("--last cannot be combined with --model")
	case flags.ifMode:
		return usageErrorf("--last cannot be combined with --if")
	case flags.unlessMode:
		return usageErrorf("--last cannot be combined with --unless")
	case flags.interactive:
		return usageErrorf("--last cannot be combined with --interactive")
	case flags.incognito:
		return usageErrorf("--last cannot be combined with --incognito")
	case flags.stats:
		return usageErrorf("--last cannot be combined with --stats")
	case flags.maxInput > 0:
		return usageErrorf("--last cannot be combined with --max-input")
	case flags.timeout > 0:
		return usageErrorf("--last cannot be combined with --timeout")
	}
	return nil
}

// humanBytes renders n with a unit appropriate to its scale. Uses binary
// units (KiB / MiB) to match the rest of qq's docs and the --max-input
// default ("200 KiB").
func humanBytes(n int64) string {
	const (
		kib = 1024
		mib = 1024 * 1024
	)
	switch {
	case n < kib:
		return fmt.Sprintf("%d bytes", n)
	case n < mib:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(kib))
	default:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(mib))
	}
}
