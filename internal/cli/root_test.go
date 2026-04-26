package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mscansian/qq/internal/client"
	"github.com/mscansian/qq/internal/config"
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

// newFakeSSEWithUsage streams content, then a final usage+finish chunk,
// matching what an include_usage-capable provider sends.
func newFakeSSEWithUsage(t *testing.T, chunks []string, promptTokens, completionTokens, cachedTokens int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flush, _ := w.(http.Flusher)
		for i, c := range chunks {
			encoded, _ := json.Marshal(c)
			finish := ""
			if i == len(chunks)-1 {
				finish = `"finish_reason":"stop"`
			}
			if finish != "" {
				fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%s},%s}]}\n\n", encoded, finish)
			} else {
				fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%s}}]}\n\n", encoded)
			}
			if flush != nil {
				flush.Flush()
			}
		}
		fmt.Fprintf(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":%d,\"completion_tokens\":%d,\"total_tokens\":%d,\"prompt_tokens_details\":{\"cached_tokens\":%d}}}\n\n",
			promptTokens, completionTokens, promptTokens+completionTokens, cachedTokens)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
}

// TestRunAskStatsFlag confirms that --stats prints a single stats line to
// stderr after a successful response, without affecting stdout or changing
// behavior when the flag is off.
func TestRunAskStatsFlag(t *testing.T) {
	cases := map[string]struct {
		stats        bool
		wantStderrIn []string
		wantStderrNo []string
	}{
		"stats on emits stats line with cached count": {
			stats: true,
			wantStderrIn: []string{
				"qq: stats:",
				"tokens=100/25 (125 total, 40 cached)",
				"model=test-model",
				"finish=stop",
			},
		},
		"stats off emits nothing": {
			stats:        false,
			wantStderrNo: []string{"qq: stats:"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			srv := newFakeSSEWithUsage(t, []string{"the answer is 42."}, 100, 25, 40)
			defer srv.Close()

			restore := setTestEnv(t, srv.URL+"/v1")
			defer restore()

			var stdout, stderr bytes.Buffer
			flags := &rootFlags{stats: tc.stats}
			if err := runAsk(context.Background(), flags, []string{"what's the answer?"}, strings.NewReader(""), &stdout, &stderr); err != nil {
				t.Fatalf("runAsk: %v", err)
			}
			if !strings.Contains(stdout.String(), "the answer is 42.") {
				t.Fatalf("prose missing from stdout: %q", stdout.String())
			}
			for _, want := range tc.wantStderrIn {
				if !strings.Contains(stderr.String(), want) {
					t.Fatalf("stderr missing %q, got:\n%s", want, stderr.String())
				}
			}
			for _, no := range tc.wantStderrNo {
				if strings.Contains(stderr.String(), no) {
					t.Fatalf("stderr unexpectedly contains %q, got:\n%s", no, stderr.String())
				}
			}
		})
	}
}

// TestFormatStats covers the conditional-field logic: tokens=unknown when
// the provider didn't report usage, `cached=` only when non-zero, `finish=`
// only when populated.
func TestFormatStats(t *testing.T) {
	cases := map[string]struct {
		resp  client.Response
		model string
		want  string
	}{
		"usage known, no cache, with finish": {
			resp: client.Response{
				Usage:        client.Usage{PromptTokens: 12, CompletionTokens: 340, TotalTokens: 352},
				UsageKnown:   true,
				FinishReason: "stop",
			},
			model: "gpt-4o-mini",
			want:  "qq: stats: tokens=12/340 (352 total) latency=1.23s model=gpt-4o-mini finish=stop",
		},
		"cache hit shows cached count": {
			resp: client.Response{
				Usage:        client.Usage{PromptTokens: 1200, CompletionTokens: 80, TotalTokens: 1280, CachedTokens: 1100},
				UsageKnown:   true,
				FinishReason: "stop",
			},
			model: "gpt-4o-mini",
			want:  "qq: stats: tokens=1200/80 (1280 total, 1100 cached) latency=1.23s model=gpt-4o-mini finish=stop",
		},
		"usage unknown omits token numbers": {
			resp:  client.Response{UsageKnown: false, FinishReason: "stop"},
			model: "gpt-4o-mini",
			want:  "qq: stats: tokens=unknown latency=1.23s model=gpt-4o-mini finish=stop",
		},
		"missing finish reason omits the field": {
			resp: client.Response{
				Usage:      client.Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3},
				UsageKnown: true,
			},
			model: "gpt-4o-mini",
			want:  "qq: stats: tokens=1/2 (3 total) latency=1.23s model=gpt-4o-mini",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := formatStats(&tc.resp, tc.model, 1230*time.Millisecond)
			if got != tc.want {
				t.Fatalf("mismatch\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

// TestRunAskInteractiveRejectsDecisionMode locks the contract that
// --interactive cannot be combined with --if/--unless. The two features
// have incompatible stdout semantics — decision mode reserves stdout for
// passthrough, interactive reserves it for confirmed prose.
func TestRunAskInteractiveRejectsDecisionMode(t *testing.T) {
	cases := map[string]rootFlags{
		"--interactive + --if":     {interactive: true, ifMode: true},
		"--interactive + --unless": {interactive: true, unlessMode: true},
	}
	for name, flags := range cases {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := runAsk(context.Background(), &flags, []string{"x"}, strings.NewReader(""), &stdout, &stderr)
			if err == nil {
				t.Fatalf("expected usage error, got nil")
			}
			var ce *cliError
			if !errors.As(err, &ce) || ce.code != exitUsage {
				t.Fatalf("expected exit code %d, got %v", exitUsage, err)
			}
			if !strings.Contains(err.Error(), "interactive") {
				t.Fatalf("expected error to mention interactive, got: %v", err)
			}
		})
	}
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

func TestResolveTimeout(t *testing.T) {
	cases := map[string]struct {
		flag    time.Duration
		profile time.Duration
		cfg     string // request.timeout in config
		want    time.Duration
	}{
		"all unset → built-in default": {want: 60 * time.Second},
		"config only":                  {cfg: "30s", want: 30 * time.Second},
		"profile beats config":         {profile: 45 * time.Second, cfg: "30s", want: 45 * time.Second},
		"flag beats profile":           {flag: 10 * time.Second, profile: 45 * time.Second, want: 10 * time.Second},
		"flag beats config":            {flag: 10 * time.Second, cfg: "30s", want: 10 * time.Second},
		"flag beats both":              {flag: 10 * time.Second, profile: 45 * time.Second, cfg: "30s", want: 10 * time.Second},
		"profile only":                 {profile: 45 * time.Second, want: 45 * time.Second},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := &config.Config{Request: config.Request{Timeout: tc.cfg}}
			got := resolveTimeout(tc.flag, tc.profile, cfg)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
