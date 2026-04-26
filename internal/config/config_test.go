package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRequestTimeoutDefault(t *testing.T) {
	c := &Config{}
	if got := c.RequestTimeout(); got != 120*time.Second {
		t.Fatalf("default: got %v, want 120s", got)
	}
}

func TestRequestTimeoutParsed(t *testing.T) {
	c := &Config{Request: Request{Timeout: "45s"}}
	if got := c.RequestTimeout(); got != 45*time.Second {
		t.Fatalf("got %v, want 45s", got)
	}
}

func TestLoadConfigTimeoutValidation(t *testing.T) {
	cases := map[string]struct {
		body    string
		wantErr string
	}{
		"valid duration": {
			body:    `[request]` + "\n" + `timeout = "45s"`,
			wantErr: "",
		},
		"invalid duration": {
			body:    `[request]` + "\n" + `timeout = "nope"`,
			wantErr: "is not a Go duration",
		},
		"non-positive duration": {
			body:    `[request]` + "\n" + `timeout = "0s"`,
			wantErr: "must be positive",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("XDG_CONFIG_HOME", dir)
			qqDir := filepath.Join(dir, "qq")
			if err := os.MkdirAll(qqDir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(qqDir, "config.toml"), []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}

			_, err := LoadConfig()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}
