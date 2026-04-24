package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/mscansian/qq/internal/config"
)

// Entry is one JSONL record. Decision is empty in normal mode. Token
// counts are omitted when the provider didn't report usage (interrupted
// stream, or a provider that doesn't honor include_usage).
type Entry struct {
	Timestamp        time.Time `json:"timestamp"`
	Profile          string    `json:"profile"`
	Model            string    `json:"model"`
	Question         string    `json:"question"`
	Answer           string    `json:"answer"`
	Decision         string    `json:"decision,omitempty"`
	PromptTokens     int64     `json:"prompt_tokens,omitempty"`
	CompletionTokens int64     `json:"completion_tokens,omitempty"`
	TotalTokens      int64     `json:"total_tokens,omitempty"`
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
	// MkdirAll is a no-op on an existing dir regardless of its mode;
	// tighten explicitly so a pre-existing state dir with wider perms
	// doesn't leak history metadata to co-tenants.
	_ = os.Chmod(dir, 0o700)

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

// minBytesPerLine is a conservative lower bound on the serialized size of
// one Entry (an all-empty JSON encoding is ~87 bytes). Used as a stat-based
// short circuit: a file under maxEntries * minBytesPerLine cannot possibly
// hold more than maxEntries lines, so rotation is a no-op.
const minBytesPerLine = 80

// rotate trims the file to the last maxEntries lines when it has more. It is
// best-effort: if two invocations rotate concurrently one may lose, which
// matches the design tradeoff in ENGINEERING.md §No file locking.
//
// Fast path: stat the file and skip everything when the size proves it's
// under cap. Slow path: scan chunks backwards from EOF to find the offset
// where the kept region begins, then copy that region into a fresh temp
// file and rename. Both avoid the full-file double-scan of the old
// countLines + tailLines pair.
func rotate(path string, maxEntries int) error {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Size() < int64(maxEntries)*minBytesPerLine {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	keepFrom, over, err := tailOffset(f, info.Size(), maxEntries)
	if err != nil {
		return err
	}
	if !over {
		return nil
	}

	if _, err := f.Seek(keepFrom, io.SeekStart); err != nil {
		return err
	}

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
	if _, err := io.Copy(tmp, f); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}

// tailOffset scans f backwards from EOF looking for the (maxEntries+1)-th
// newline. If it finds one, returns the byte offset that begins the
// kept region (the byte just after that newline) and over=true. If it
// reaches BOF first, the file has ≤ maxEntries entries and no rotation is
// needed; returns (0, false, nil).
//
// Assumes every entry ends with '\n' — which is true for anything Append
// writes.
func tailOffset(f *os.File, size int64, maxEntries int) (int64, bool, error) {
	const chunk = 64 * 1024
	target := maxEntries + 1
	buf := make([]byte, chunk)
	pos := size
	seen := 0
	for pos > 0 {
		read := min(int64(chunk), pos)
		pos -= read
		if _, err := f.ReadAt(buf[:read], pos); err != nil {
			return 0, false, err
		}
		for i := int(read) - 1; i >= 0; i-- {
			if buf[i] != '\n' {
				continue
			}
			seen++
			if seen == target {
				return pos + int64(i) + 1, true, nil
			}
		}
	}
	return 0, false, nil
}
