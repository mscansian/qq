package config

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestCredentialsParsing(t *testing.T) {
	cases := map[string]struct {
		toml       string
		wantErr    string // substring
		wantCount  int
		assertFunc func(t *testing.T, c map[string]Profile)
	}{
		"single profile": {
			toml: `
[default]
base_url = "https://api.openai.com/v1"
api_key = "sk-abc"
model = "gpt-5.4-mini"
`,
			wantCount: 1,
			assertFunc: func(t *testing.T, c map[string]Profile) {
				if c["default"].APIKey != "sk-abc" {
					t.Fatalf("got api_key=%q", c["default"].APIKey)
				}
			},
		},
		"incognito honored": {
			toml: `
[work]
base_url = "https://api.openai.com/v1"
api_key = "sk-x"
model = "gpt-5.4-mini"
incognito = true
`,
			wantCount: 1,
			assertFunc: func(t *testing.T, c map[string]Profile) {
				if !c["work"].Incognito {
					t.Fatal("want incognito=true")
				}
			},
		},
		"unknown field rejected": {
			toml: `
[default]
base_url = "x"
api_key = "y"
model = "z"
not_a_field = true
`,
			wantErr: "strict mode",
		},
		"system_prompt optional": {
			toml: `
[translate]
base_url = "https://api/"
api_key = "k"
model = "m"
system_prompt = "Translate to English"
`,
			wantCount: 1,
			assertFunc: func(t *testing.T, c map[string]Profile) {
				if c["translate"].SystemPrompt != "Translate to English" {
					t.Fatal("system_prompt not parsed")
				}
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var got map[string]Profile
			dec := toml.NewDecoder(bytes.NewReader([]byte(tc.toml)))
			dec.DisallowUnknownFields()
			err := dec.Decode(&got)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want error %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if len(got) != tc.wantCount {
				t.Fatalf("want %d profiles, got %d", tc.wantCount, len(got))
			}
			if tc.assertFunc != nil {
				tc.assertFunc(t, got)
			}
		})
	}
}

func TestLoadCredentialsMissingFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	c, err := LoadCredentials()
	if err != nil {
		t.Fatalf("unexpected error on missing file: %v", err)
	}
	if c.Exists {
		t.Fatal("want Exists=false")
	}
	if len(c.Profiles) != 0 {
		t.Fatalf("want empty profiles, got %d", len(c.Profiles))
	}
}

func TestLoadCredentialsModeWarning(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	qqDir := filepath.Join(dir, "qq")
	if err := os.MkdirAll(qqDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(qqDir, "credentials.toml")
	if err := os.WriteFile(path, []byte("[default]\nbase_url=\"x\"\napi_key=\"y\"\nmodel=\"z\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := LoadCredentials()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.ModeWarning == "" {
		t.Fatal("want a ModeWarning for 0644 file")
	}
}

func TestSaveWritesPermissions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	c := &Credentials{
		Path: filepath.Join(dir, "qq", "credentials.toml"),
		Profiles: map[string]Profile{
			"default": {BaseURL: "https://api/", APIKey: "k", Model: "m"},
		},
	}
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(c.Path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("want 0600, got %o", info.Mode().Perm())
	}
}
