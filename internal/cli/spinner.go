package cli

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

// spinner writes a single spinning glyph to stderr while waiting for the
// first token. It is a no-op unless stderr is a TTY — so `qq "..." 2>log`
// never writes glyphs into the log file.
type spinner struct {
	w    io.Writer
	stop func()
	once sync.Once
}

var spinnerFrames = []rune{'|', '/', '-', '\\'}

func startSpinner(stderr io.Writer) *spinner {
	s := &spinner{w: stderr}
	if !isStderrTTY(stderr) {
		s.stop = func() {}
		return s
	}
	done := make(chan struct{})
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		i := 0
		for {
			select {
			case <-done:
				return
			case <-time.After(80 * time.Millisecond):
				fmt.Fprintf(stderr, "\r%c", spinnerFrames[i%len(spinnerFrames)])
				i++
			}
		}
	}()
	s.stop = func() {
		s.once.Do(func() {
			// wait for the ticker goroutine to exit before writing the
			// clear sequence, or a glyph printed after close(done) can
			// land after our clear and stick around on screen.
			close(done)
			<-exited
			fmt.Fprint(stderr, "\r \r")
		})
	}
	return s
}

// clear is called on the first stdout write. It simply tears down the
// spinner (same as stop) — separate method to read more clearly at the
// call site.
func (s *spinner) clear() { s.stop() }

func isStderrTTY(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}
