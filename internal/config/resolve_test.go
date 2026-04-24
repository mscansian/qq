package config

import (
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	t.Setenv("QQ_API_KEY", "")
	t.Setenv("QQ_BASE_URL", "")
	t.Setenv("QQ_MODEL", "")
	t.Setenv("QQ_PROFILE", "")

	fullDefault := func() *Credentials {
		return &Credentials{
			Profiles: map[string]Profile{
				"default": {BaseURL: "https://api/", APIKey: "k1", Model: "m1"},
				"grok":    {BaseURL: "https://x.ai/", APIKey: "k2", Model: "m2"},
			},
		}
	}

	cases := map[string]struct {
		creds     func() *Credentials
		env       map[string]string
		ov        Overrides
		wantErr   string // substring; empty means no error
		wantKey   string
		wantURL   string
		wantModel string
		wantName  string
	}{
		"uses default profile": {
			creds:     fullDefault,
			wantKey:   "k1",
			wantURL:   "https://api/",
			wantModel: "m1",
			wantName:  "default",
		},
		"flag picks profile": {
			creds:     fullDefault,
			ov:        Overrides{Profile: "grok"},
			wantKey:   "k2",
			wantURL:   "https://x.ai/",
			wantModel: "m2",
			wantName:  "grok",
		},
		"env picks profile": {
			creds:     fullDefault,
			env:       map[string]string{"QQ_PROFILE": "grok"},
			wantKey:   "k2",
			wantURL:   "https://x.ai/",
			wantModel: "m2",
			wantName:  "grok",
		},
		"flag beats env": {
			creds:     fullDefault,
			env:       map[string]string{"QQ_PROFILE": "grok"},
			ov:        Overrides{Profile: "default"},
			wantKey:   "k1",
			wantURL:   "https://api/",
			wantModel: "m1",
			wantName:  "default",
		},
		"model flag overrides profile": {
			creds:     fullDefault,
			ov:        Overrides{Model: "gpt-x"},
			wantKey:   "k1",
			wantURL:   "https://api/",
			wantModel: "gpt-x",
			wantName:  "default",
		},
		"QQ_MODEL overrides profile but flag still wins": {
			creds:     fullDefault,
			env:       map[string]string{"QQ_MODEL": "m-env"},
			wantKey:   "k1",
			wantURL:   "https://api/",
			wantModel: "m-env",
			wantName:  "default",
		},
		"env-only config works without any profile": {
			creds: func() *Credentials { return &Credentials{Profiles: map[string]Profile{}} },
			env: map[string]string{
				"QQ_API_KEY":  "ek",
				"QQ_BASE_URL": "https://env/",
				"QQ_MODEL":    "e-m",
			},
			wantKey:   "ek",
			wantURL:   "https://env/",
			wantModel: "e-m",
			wantName:  "",
		},
		"unknown profile errors": {
			creds:   fullDefault,
			ov:      Overrides{Profile: "nope"},
			wantErr: "profile \"nope\" not found",
		},
		"no profile no env errors with remediation": {
			creds:   func() *Credentials { return &Credentials{Profiles: map[string]Profile{}} },
			wantErr: "no default profile found",
		},
		"profile missing field is surfaced": {
			creds: func() *Credentials {
				return &Credentials{Profiles: map[string]Profile{
					"default": {BaseURL: "https://api/", Model: "m1"}, // no api_key
				}}
			},
			wantErr: "missing api_key",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			got, err := Resolve(tc.creds(), tc.ov)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.APIKey != tc.wantKey || got.BaseURL != tc.wantURL || got.Model != tc.wantModel || got.ProfileName != tc.wantName {
				t.Fatalf("got %+v, want key=%s url=%s model=%s name=%s",
					got, tc.wantKey, tc.wantURL, tc.wantModel, tc.wantName)
			}
		})
	}
}
