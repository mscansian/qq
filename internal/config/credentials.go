package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Profile is one entry in credentials.toml.
type Profile struct {
	BaseURL      string `toml:"base_url"`
	APIKey       string `toml:"api_key"`
	Model        string `toml:"model"`
	SystemPrompt string `toml:"system_prompt,omitempty"`
	Incognito    bool   `toml:"incognito,omitempty"`
	Timeout      string `toml:"timeout,omitempty"`     // Go duration string; "" → fall through to config.toml
	MaxBytes     int    `toml:"max_bytes,omitempty"`   // 0 → fall through to config.toml
	OnOverflow   string `toml:"on_overflow,omitempty"` // "error" or "truncate"; "" → fall through to config.toml
}

// Credentials is the parsed credentials.toml — a map of profile name → Profile.
type Credentials struct {
	Profiles map[string]Profile
	// Path is where the file was loaded from (or would be written to).
	Path string
	// Exists is false if the file was not present on disk.
	Exists bool
	// ModeWarning is set if the file permissions are wider than 0600.
	ModeWarning string
}

// CredentialsPath returns the path to credentials.toml under $XDG_CONFIG_HOME
// (or ~/.config), creating neither the file nor the directory.
func CredentialsPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.toml"), nil
}

// LoadCredentials reads credentials.toml. A missing file is not an error —
// the returned Credentials has Exists=false and an empty profile map.
func LoadCredentials() (*Credentials, error) {
	path, err := CredentialsPath()
	if err != nil {
		return nil, err
	}

	c := &Credentials{
		Profiles: map[string]Profile{},
		Path:     path,
	}

	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	c.Exists = true

	// Permission check: warn if readable by group or other.
	if info.Mode().Perm()&0o077 != 0 {
		c.ModeWarning = fmt.Sprintf("qq: warning: %s has permissions %o; recommend 0600", path, info.Mode().Perm())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	raw := map[string]Profile{}
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	c.Profiles = raw
	return c, nil
}

// Save writes credentials to disk with 0600 permissions, creating the parent
// directory with 0700 if needed.
func (c *Credentials) Save() error {
	dir := filepath.Dir(c.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	// Tighten the directory in case it existed with wider perms.
	_ = os.Chmod(dir, 0o700)

	buf, err := toml.Marshal(c.Profiles)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	// Write atomically: temp file in same directory, fsync, rename.
	tmp, err := os.CreateTemp(dir, ".credentials.toml.*")
	if err != nil {
		return fmt.Errorf("tempfile: %w", err)
	}
	tmpName := tmp.Name()
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if _, err := tmp.Write(buf); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, c.Path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename %s: %w", c.Path, err)
	}
	return nil
}
