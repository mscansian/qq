package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestRunAskDecisionPassthrough wires runAsk end-to-end against a fake SSE
// server and verifies the decision-mode passthrough contract: on a gate-open
// verdict the buffered stdin is echoed to stdout; on no/unknown stdout stays
// empty; the prose always lands on stderr regardless of verdict.
func TestRunAskDecisionPassthrough(t *testing.T) {
	cases := map[string]struct {
		ifMode       bool
		unlessMode   bool
		chunks       []string
		arg          string
		stdin        string
		wantStdout   string
		wantStderrIn []string // substrings that must appear on stderr
	}{
		"if yes → stdin passes through": {
			ifMode:       true,
			chunks:       []string{"yes\n\nLooks clean."},
			arg:          "is this safe?",
			stdin:        "the original payload",
			wantStdout:   "the original payload",
			wantStderrIn: []string{"Looks clean."},
		},
		"if no → stdout empty, exit 1": {
			ifMode:       true,
			chunks:       []string{"no\n\nNope, found a problem."},
			arg:          "is this safe?",
			stdin:        "the original payload",
			wantStdout:   "",
			wantStderrIn: []string{"Nope, found a problem."},
		},
		"unless no → stdin passes through": {
			unlessMode:   true,
			chunks:       []string{"no\n\nClean diff."},
			arg:          "any debug prints?",
			stdin:        "diff --git a/x b/x",
			wantStdout:   "diff --git a/x b/x",
			wantStderrIn: []string{"Clean diff."},
		},
		"unless yes → stdout empty, exit 1": {
			unlessMode:   true,
			chunks:       []string{"yes\n\nFound a console.log."},
			arg:          "any debug prints?",
			stdin:        "console.log('x')",
			wantStdout:   "",
			wantStderrIn: []string{"Found a console.log."},
		},
		"unknown → stdout empty regardless of mode": {
			ifMode:       true,
			chunks:       []string{"unknown\n\nNot enough info."},
			arg:          "should I roll back?",
			stdin:        "incident log contents",
			wantStdout:   "",
			wantStderrIn: []string{"Not enough info."},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv := newFakeSSE(t, tc.chunks)
			defer srv.Close()

			restore := setTestEnv(t, srv.URL+"/v1")
			defer restore()

			flags := &rootFlags{
				ifMode:     tc.ifMode,
				unlessMode: tc.unlessMode,
			}

			var stdout, stderr bytes.Buffer
			err := runAsk(context.Background(), flags, []string{tc.arg}, strings.NewReader(tc.stdin), &stdout, &stderr)

			if stdout.String() != tc.wantStdout {
				t.Fatalf("stdout mismatch\n got: %q\nwant: %q", stdout.String(), tc.wantStdout)
			}
			for _, want := range tc.wantStderrIn {
				if !strings.Contains(stderr.String(), want) {
					t.Fatalf("stderr missing %q, got:\n%s", want, stderr.String())
				}
			}

			// Exit semantics are carried via cliError.code; empty stdout
			// on the "no verdict" cases should correspond to a non-zero
			// exit error returned from runAsk.
			if tc.wantStdout == "" && err == nil {
				t.Fatalf("expected non-zero exit error, got nil")
			}
			if tc.wantStdout != "" && err != nil {
				t.Fatalf("expected zero exit error, got %v", err)
			}
		})
	}
}

// TestRunAskNormalModeStdout confirms the normal (non-decision) path still
// writes model prose to stdout and leaves stderr free of prose.
func TestRunAskNormalModeStdout(t *testing.T) {
	srv := newFakeSSE(t, []string{"curl follows redirects with -L."})
	defer srv.Close()

	restore := setTestEnv(t, srv.URL+"/v1")
	defer restore()

	flags := &rootFlags{}
	var stdout, stderr bytes.Buffer
	if err := runAsk(context.Background(), flags, []string{"how to follow redirects?"}, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("runAsk: %v", err)
	}
	if !strings.Contains(stdout.String(), "curl follows redirects with -L.") {
		t.Fatalf("expected prose on stdout, got %q", stdout.String())
	}
	if strings.Contains(stderr.String(), "curl follows redirects with -L.") {
		t.Fatalf("prose leaked to stderr: %q", stderr.String())
	}
}

func newFakeSSE(t *testing.T, chunks []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flush, _ := w.(http.Flusher)
		for _, c := range chunks {
			encoded, _ := json.Marshal(c)
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%s}}]}\n\n", encoded)
			if flush != nil {
				flush.Flush()
			}
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
}

// setTestEnv isolates the test from the user's real config and points qq at
// the fake server via the env-var-only path. The returned function restores
// nothing — t.Setenv handles reset after the test.
func setTestEnv(t *testing.T, baseURL string) func() {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("QQ_API_KEY", "test-key")
	t.Setenv("QQ_BASE_URL", baseURL)
	t.Setenv("QQ_MODEL", "test-model")
	t.Setenv("QQ_PROFILE", "")
	return func() {}
}
