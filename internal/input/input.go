package input

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// DefaultMaxInput is the default stdin cap (200 KiB).
const DefaultMaxInput = 200 * 1024

// Result is the outcome of resolving CLI args + stdin into a single user
// message plus a "question" string for history.
type Result struct {
	// UserMessage is the fully-composed text sent to the model.
	UserMessage string
	// Question is the human-readable form recorded in history: argument if
	// present, else stdin, without the wrapping tags.
	Question string
	// Truncated is true if stdin hit the size cap.
	Truncated bool
	// Source explains where the content came from, for error messages.
	Source string
	// ContentTag is the per-invocation tag name used to wrap stdin when arg
	// and stdin were both present. Empty when no wrapping happened. The
	// caller mirrors it into the system prompt so the model knows which
	// region is untrusted.
	ContentTag string
}

// ErrNoInput is returned when there's neither an argument nor piped stdin.
var ErrNoInput = errors.New("no input: pass a question as an argument or pipe content to stdin")

// Options controls Resolve behavior. Fields map 1-to-1 to CLI flags.
type Options struct {
	// Arg is the positional argument (or "-" to mean "read from stdin
	// explicitly"). Empty string means no arg was given.
	Arg string
	// ArgGiven distinguishes an explicit empty-string argument from no arg
	// being present at all. Cobra doesn't let you pass an empty positional,
	// so this mostly matches Arg != "", but we make it explicit for clarity
	// in tests.
	ArgGiven bool
	// MaxInput caps stdin bytes. 0 → DefaultMaxInput.
	MaxInput int
	// Stdin is the reader to consume. Injected for testing. When nil,
	// os.Stdin is used.
	Stdin io.Reader
	// StdinIsTerminal reports whether the caller's stdin is a TTY. When
	// nil, Resolve checks os.Stdin directly.
	StdinIsTerminal func() bool
	// Nonce overrides the random suffix used in the content wrapping tag.
	// Tests inject a deterministic value; production leaves it empty and
	// Resolve generates 64 fresh random bits.
	Nonce string
}

// Resolve applies the three input shapes documented in ENGINEERING.md
// §Input handling: arg only, stdin only, arg + stdin.
func Resolve(opts Options) (*Result, error) {
	max := opts.MaxInput
	if max <= 0 {
		max = DefaultMaxInput
	}

	stdin := opts.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}

	isTerm := opts.StdinIsTerminal
	if isTerm == nil {
		isTerm = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }
	}

	arg := opts.Arg
	explicitStdin := arg == "-"
	hasArg := opts.ArgGiven && !explicitStdin

	// Decide whether to read stdin.
	readStdin := explicitStdin || !isTerm()

	var payload string
	var truncated bool
	if readStdin {
		b, tr, err := readCapped(stdin, max)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
		payload = string(b)
		truncated = tr
	}

	switch {
	case hasArg && readStdin && payload != "":
		nonce := opts.Nonce
		if nonce == "" {
			n, err := newNonce()
			if err != nil {
				return nil, fmt.Errorf("generate nonce: %w", err)
			}
			nonce = n
		}
		tag := "content-" + nonce
		escaped := escapeContent(payload, tag)
		msg := fmt.Sprintf("%s\n\n<%s>\n%s\n</%s>", arg, tag, escaped, tag)
		return &Result{
			UserMessage: msg,
			Question:    arg,
			Truncated:   truncated,
			Source:      "arg+stdin",
			ContentTag:  tag,
		}, nil

	case hasArg:
		return &Result{
			UserMessage: arg,
			Question:    arg,
			Source:      "arg",
		}, nil

	case readStdin && payload != "":
		// Stdin-only: the payload is the entire user message. We do not
		// wrap in <content> tags here — with no accompanying instruction,
		// the payload IS the instruction, and wrapping would confuse the
		// model about what we want.
		return &Result{
			UserMessage: payload,
			Question:    payload,
			Truncated:   truncated,
			Source:      "stdin",
		}, nil

	default:
		return nil, ErrNoInput
	}
}

// readCapped reads up to max+1 bytes from r, returning the first max and
// whether truncation occurred (more than max bytes were available).
func readCapped(r io.Reader, max int) ([]byte, bool, error) {
	buf := make([]byte, max+1)
	n, err := io.ReadFull(r, buf)
	switch {
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return buf[:n], false, nil
	case err != nil:
		return nil, false, err
	default:
		// n == max+1: at least one byte over the cap.
		// Drain the rest to keep writers from blocking, but discard it.
		_, _ = io.Copy(io.Discard, r)
		return buf[:max], true, nil
	}
}

// escapeContent neutralizes literal opens/closes of the wrapping tag inside
// the payload so an attacker can't close our data region early. The tag name
// already carries a per-invocation random nonce, so a forged close requires
// guessing the nonce; this replacement is belt-and-suspenders for the
// astronomically unlikely collision.
func escapeContent(s, tag string) string {
	startTag := "<" + tag + ">"
	endTag := "</" + tag + ">"
	s = strings.ReplaceAll(s, endTag, "<\\/"+tag+">")
	s = strings.ReplaceAll(s, startTag, "<\\"+tag+">")
	return s
}

// newNonce returns 64 random bits as a 16-char hex string, enough that a
// forged </content-NONCE> close inside a payload is negligibly likely.
func newNonce() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
