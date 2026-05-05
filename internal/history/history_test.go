package history

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendAndRotate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	for i := range 5 {
		if err := Append(Entry{
			Timestamp: time.Unix(int64(i), 0).UTC(),
			Profile:   "default",
			Model:     "m",
			Question:  "q",
			Answer:    "a",
		}, 3); err != nil {
			t.Fatal(err)
		}
	}

	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var entries []Entry
	for sc.Scan() {
		var e Entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, e)
	}
	if len(entries) != 3 {
		t.Fatalf("want 3 entries after rotation, got %d", len(entries))
	}
	// Rotation must keep the newest entries (timestamps 2, 3, 4), not the
	// oldest — this pins the tail direction.
	for i, want := range []int64{2, 3, 4} {
		if got := entries[i].Timestamp.Unix(); got != want {
			t.Fatalf("entry %d: want ts=%d, got %d", i, want, got)
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("file should not be readable by group/other, got %o", info.Mode().Perm())
	}
}

func TestRotateSpansMultipleChunks(t *testing.T) {
	// Entries padded so the file crosses the 64 KiB backward-scan chunk,
	// exercising the case where tailOffset has to read more than one chunk
	// before finding the (maxEntries+1)-th newline.
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	padding := make([]byte, 2048)
	for i := range padding {
		padding[i] = 'x'
	}
	const total = 80
	const cap = 5
	for i := range total {
		if err := Append(Entry{
			Timestamp: time.Unix(int64(i), 0).UTC(),
			Question:  "q",
			Answer:    string(padding),
		}, cap); err != nil {
			t.Fatal(err)
		}
	}

	path, _ := Path()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := bytes.Split(bytes.TrimRight(data, "\n"), []byte{'\n'})
	if len(lines) != cap {
		t.Fatalf("want %d lines, got %d", cap, len(lines))
	}
	var first, last Entry
	if err := json.Unmarshal(lines[0], &first); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(lines[len(lines)-1], &last); err != nil {
		t.Fatal(err)
	}
	if first.Timestamp.Unix() != total-cap {
		t.Fatalf("first kept entry: want ts=%d, got %d", total-cap, first.Timestamp.Unix())
	}
	if last.Timestamp.Unix() != total-1 {
		t.Fatalf("last kept entry: want ts=%d, got %d", total-1, last.Timestamp.Unix())
	}
}

func TestLastReturnsErrNoHistoryWhenMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	_, err := Last()
	if !errors.Is(err, ErrNoHistory) {
		t.Fatalf("want ErrNoHistory, got %v", err)
	}
}

func TestLastReturnsErrNoHistoryWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = Last()
	if !errors.Is(err, ErrNoHistory) {
		t.Fatalf("want ErrNoHistory, got %v", err)
	}
}

func TestLastReturnsSingleEntry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	want := Entry{
		Timestamp:    time.Unix(42, 0).UTC(),
		Profile:      "default",
		Model:        "m",
		Question:     "what is DNS?",
		Answer:       "name resolution.",
		PayloadBytes: 1234,
	}
	if err := Append(want, 0); err != nil {
		t.Fatal(err)
	}

	got, err := Last()
	if err != nil {
		t.Fatal(err)
	}
	if got.Question != want.Question || got.Answer != want.Answer || got.PayloadBytes != want.PayloadBytes {
		t.Fatalf("round trip mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

func TestLastReturnsMostRecentOfMany(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	for i := range 5 {
		if err := Append(Entry{
			Timestamp: time.Unix(int64(i), 0).UTC(),
			Question:  "q",
			Answer:    fmt.Sprintf("answer-%d", i),
		}, 0); err != nil {
			t.Fatal(err)
		}
	}

	got, err := Last()
	if err != nil {
		t.Fatal(err)
	}
	if got.Answer != "answer-4" {
		t.Fatalf("want last answer 'answer-4', got %q", got.Answer)
	}
}

func TestAppendNoRotateWhenUnderCap(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	for range 2 {
		if err := Append(Entry{Question: "q", Answer: "a"}, 1000); err != nil {
			t.Fatal(err)
		}
	}

	path, _ := Path()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(data, []byte{'\n'}); n != 2 {
		t.Fatalf("want 2 lines, got %d", n)
	}
}
