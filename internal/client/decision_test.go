package client

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

func TestProcessorNormalMode(t *testing.T) {
	var out, errOut bytes.Buffer
	p := newProcessor(&out, &errOut, false)

	io.WriteString(p, "hello ")
	io.WriteString(p, "world")
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "hello world\n" {
		t.Fatalf("got %q", out.String())
	}
	if _, ok := p.Decision(); ok {
		t.Fatal("normal mode should not report a decision")
	}
	if p.Prose() != "hello world" {
		t.Fatalf("prose mismatch: %q", p.Prose())
	}
}

func TestProcessorDecisionMode(t *testing.T) {
	cases := map[string]struct {
		chunks       []string
		wantDecision Decision
		wantStdout   string
		wantStderr   string // substring
	}{
		"simple yes": {
			chunks:       []string{"yes\n\nThe build looks fine."},
			wantDecision: DecisionYes,
			wantStdout:   "The build looks fine.\n",
		},
		"no with punctuation": {
			chunks:       []string{"No.\n\nThe change is risky."},
			wantDecision: DecisionNo,
			wantStdout:   "The change is risky.\n",
		},
		"uppercase unknown": {
			chunks:       []string{"UNKNOWN\n\nNot enough info."},
			wantDecision: DecisionUnknown,
			wantStdout:   "Not enough info.\n",
		},
		"off-format treated as unknown": {
			chunks:       []string{"42\n\nI cannot answer that."},
			wantDecision: DecisionUnknown,
			wantStdout:   "I cannot answer that.\n",
			wantStderr:   "didn't follow decision format",
		},
		"split across chunks before newline": {
			chunks:       []string{"ye", "s", "\n\n", "Looks ", "good."},
			wantDecision: DecisionYes,
			wantStdout:   "Looks good.\n",
		},
		"no newline at all — full response is treated as decision": {
			chunks:       []string{"yes"},
			wantDecision: DecisionYes,
			wantStdout:   "\n",
		},
		"missing blank separator tolerated": {
			chunks:       []string{"yes\nbody right after"},
			wantDecision: DecisionYes,
			wantStdout:   "body right after\n",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			p := newProcessor(&out, &errOut, true)
			for _, c := range tc.chunks {
				if _, err := p.Write([]byte(c)); err != nil {
					t.Fatal(err)
				}
			}
			if err := p.Close(); err != nil {
				t.Fatal(err)
			}
			d, ok := p.Decision()
			if !ok || d != tc.wantDecision {
				t.Fatalf("decision: got %q ok=%v, want %q", d, ok, tc.wantDecision)
			}
			if out.String() != tc.wantStdout {
				t.Fatalf("stdout mismatch\n got: %q\nwant: %q", out.String(), tc.wantStdout)
			}
			if tc.wantStderr != "" && !strings.Contains(errOut.String(), tc.wantStderr) {
				t.Fatalf("stderr mismatch\n got: %q\nwant substring: %q", errOut.String(), tc.wantStderr)
			}
		})
	}
}

func TestComposeSystemPrompt(t *testing.T) {
	cases := map[string]struct {
		base     string
		decision bool
		wantHas  string
		wantNot  string
	}{
		"default prompt without decision": {
			wantHas: "terminal assistant",
		},
		"profile override replaces default": {
			base:    "Custom prompt.",
			wantHas: "Custom prompt.",
			wantNot: "terminal assistant",
		},
		"decision appends format block": {
			decision: true,
			wantHas:  "FIRST LINE",
		},
		"decision composes on top of profile override": {
			base:     "Translate to English.",
			decision: true,
			wantHas:  "FIRST LINE",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ComposeSystemPrompt(tc.base, tc.decision)
			if tc.wantHas != "" && !strings.Contains(got, tc.wantHas) {
				t.Fatalf("missing %q in\n%s", tc.wantHas, got)
			}
			if tc.wantNot != "" && strings.Contains(got, tc.wantNot) {
				t.Fatalf("should not contain %q", tc.wantNot)
			}
		})
	}
}
