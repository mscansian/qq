package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestRunAgainstFakeSSE exercises the end-to-end streaming path against a
// handler that speaks OpenAI-compatible SSE. Covers: request shape, delta
// reassembly, control-byte filter, decision state machine, and history
// prose extraction — enough that regressions in any one of those would
// surface here rather than only in unit tests.
func TestRunAgainstFakeSSE(t *testing.T) {
	cases := map[string]struct {
		decision     bool
		chunks       []string
		wantStdout   string
		wantProse    string
		wantDecision Decision
	}{
		"normal streaming": {
			chunks:     []string{"Hello ", "world."},
			wantStdout: "Hello world.\n",
			wantProse:  "Hello world.",
		},
		"control bytes stripped": {
			chunks:     []string{"Hel\x1b[31mlo", " world\x07."},
			wantStdout: "Hel[31mlo world.\n",
			wantProse:  "Hel[31mlo world.",
		},
		"decision yes": {
			decision:     true,
			chunks:       []string{"yes\n\n", "Looks ", "good."},
			wantStdout:   "Looks good.\n",
			wantProse:    "Looks good.",
			wantDecision: DecisionYes,
		},
		"decision with off-format parses as unknown": {
			decision:     true,
			chunks:       []string{"42\n\nI don't know."},
			wantStdout:   "I don't know.\n",
			wantProse:    "I don't know.",
			wantDecision: DecisionUnknown,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// base URL in this test includes a /v1 path segment with
				// no trailing slash, so we require the full path below —
				// regression guard for the base URL normalization fix.
				if r.URL.Path != "/v1/chat/completions" {
					t.Errorf("unexpected path %q", r.URL.Path)
				}
				if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer ") {
					t.Errorf("missing bearer token header, got %q", got)
				}
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				flush, _ := w.(http.Flusher)
				for _, c := range tc.chunks {
					encoded, _ := json.Marshal(c)
					fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%s}}]}\n\n", encoded)
					if flush != nil {
						flush.Flush()
					}
				}
				fmt.Fprint(w, "data: [DONE]\n\n")
			}))
			defer srv.Close()

			var out, errOut bytes.Buffer
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			resp, err := Run(ctx, Request{
				BaseURL:      srv.URL + "/v1", // no trailing slash, on purpose
				APIKey:       "test-key",
				Model:        "test-model",
				SystemPrompt: ComposeSystemPrompt("", "", tc.decision),
				UserMessage:  "hi",
				Decision:     tc.decision,
			}, &out, &errOut)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}

			if out.String() != tc.wantStdout {
				t.Fatalf("stdout mismatch\n got: %q\nwant: %q", out.String(), tc.wantStdout)
			}
			if resp.Prose != tc.wantProse {
				t.Fatalf("prose mismatch\n got: %q\nwant: %q", resp.Prose, tc.wantProse)
			}
			if tc.decision && resp.Decision != tc.wantDecision {
				t.Fatalf("decision mismatch: got %q want %q", resp.Decision, tc.wantDecision)
			}
		})
	}
}
