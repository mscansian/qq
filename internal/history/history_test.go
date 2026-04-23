package history

import (
	"bufio"
	"os"
	"testing"
	"time"
)

func TestAppendAndRotate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	for i := 0; i < 5; i++ {
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
	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if len(lines) != 3 {
		t.Fatalf("want 3 lines after rotation, got %d", len(lines))
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("file should not be readable by group/other, got %o", info.Mode().Perm())
	}
}

func TestAppendNoRotateWhenUnderCap(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	for i := 0; i < 2; i++ {
		if err := Append(Entry{Question: "q", Answer: "a"}, 1000); err != nil {
			t.Fatal(err)
		}
	}

	path, _ := Path()
	n, err := countLines(path)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("want 2 lines, got %d", n)
	}
}
