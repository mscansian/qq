package history

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/mscansian/qq/internal/config"
)

// Entry is one JSONL record. Decision is empty in normal mode.
type Entry struct {
	Timestamp time.Time `json:"timestamp"`
	Profile   string    `json:"profile"`
	Model     string    `json:"model"`
	Question  string    `json:"question"`
	Answer    string    `json:"answer"`
	Decision  string    `json:"decision,omitempty"`
}

// Path returns the history.jsonl path under XDG_STATE_HOME.
func Path() (string, error) {
	dir, err := config.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "history.jsonl"), nil
}

// Append writes e followed by rotation if maxEntries is exceeded. Uses
// O_APPEND so concurrent writers on POSIX don't interleave within lines
// (writes under PIPE_BUF are atomic).
func Append(e Entry, maxEntries int) error {
	path, err := Path()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}
	line = append(line, '\n')

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	if _, err := f.Write(line); err != nil {
		f.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return err
	}

	if maxEntries <= 0 {
		return nil
	}
	return rotate(path, maxEntries)
}

// rotate trims the file to the last maxEntries lines when it has more. It is
// best-effort: if two invocations rotate concurrently one may lose, which
// matches the design tradeoff in ENGINEERING.md §No file locking.
func rotate(path string, maxEntries int) error {
	n, err := countLines(path)
	if err != nil {
		return err
	}
	if n <= maxEntries {
		return nil
	}

	keep, err := tailLines(path, maxEntries)
	if err != nil {
		return err
	}

	// Write atomically via temp file to avoid leaving a truncated history
	// on a crash partway through.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".history.*.jsonl")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	for _, ln := range keep {
		if _, err := tmp.Write(ln); err != nil {
			tmp.Close()
			os.Remove(name)
			return err
		}
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}

func countLines(path string) (int, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	n := 0
	for sc.Scan() {
		n++
	}
	return n, sc.Err()
}

// tailLines returns the last n lines of path, each with its trailing newline
// preserved so callers can write them back directly.
func tailLines(path string, n int) ([][]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)

	buf := make([][]byte, 0, n)
	for sc.Scan() {
		line := append(sc.Bytes(), '\n')
		cp := make([]byte, len(line))
		copy(cp, line)
		if len(buf) == n {
			buf = append(buf[1:], cp)
		} else {
			buf = append(buf, cp)
		}
	}
	return buf, sc.Err()
}
