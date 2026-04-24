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

// History settings (non-secret).
type History struct {
	Enabled    *bool `toml:"enabled,omitempty"`     // pointer to distinguish unset from false
	MaxEntries int   `toml:"max_entries,omitempty"` // 0 → default (1000)
}

// Input settings (non-secret).
type Input struct {
	MaxBytes   int    `toml:"max_bytes,omitempty"`   // 0 → package default in internal/input
	OnOverflow string `toml:"on_overflow,omitempty"` // "error" (default) or "truncate"
}

// OnOverflow values.
const (
	OnOverflowTruncate = "truncate"
	OnOverflowError    = "error"
)

// Config is the parsed config.toml.
type Config struct {
	History History `toml:"history"`
	Input   Input   `toml:"input"`
}

const (
	defaultMaxHistory = 1000
)

// HistoryEnabled honors the default (true) when unset.
func (c *Config) HistoryEnabled() bool {
	if c.History.Enabled == nil {
		return true
	}
	return *c.History.Enabled
}

// HistoryMaxEntries honors the default (1000) when unset.
func (c *Config) HistoryMaxEntries() int {
	if c.History.MaxEntries <= 0 {
		return defaultMaxHistory
	}
	return c.History.MaxEntries
}

// InputMaxBytes returns the configured stdin cap, or 0 when unset.
// Callers treat 0 as "use the internal/input package default", keeping
// config free of a dependency on that package.
func (c *Config) InputMaxBytes() int {
	if c.Input.MaxBytes <= 0 {
		return 0
	}
	return c.Input.MaxBytes
}

// InputOnOverflow returns the configured oversize strategy, defaulting to
// "error" when unset — a truncated verdict is judged on a prefix the user
// may not notice is clipped, so failing loud is the safer default. Opt in
// to "truncate" explicitly when you know the prefix is enough.
func (c *Config) InputOnOverflow() string {
	if c.Input.OnOverflow == "" {
		return OnOverflowError
	}
	return c.Input.OnOverflow
}

// ConfigPath returns the path to config.toml under $XDG_CONFIG_HOME
// (or ~/.config).
func ConfigPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// LoadConfig reads config.toml. A missing file returns a zero-value Config,
// not an error — config.toml is optional.
func LoadConfig() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var c Config
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	switch c.Input.OnOverflow {
	case "", OnOverflowTruncate, OnOverflowError:
	default:
		return nil, fmt.Errorf(
			"parse %s: input.on_overflow must be %q or %q, got %q",
			path, OnOverflowTruncate, OnOverflowError, c.Input.OnOverflow,
		)
	}
	return &c, nil
}

// configDir resolves the qq config directory, honoring XDG_CONFIG_HOME with
// a fallback to ~/.config.
func configDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "qq"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory: %w", err)
	}
	return filepath.Join(home, ".config", "qq"), nil
}

// StateDir resolves the qq state directory, honoring XDG_STATE_HOME with a
// fallback to ~/.local/state.
func StateDir() (string, error) {
	if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
		return filepath.Join(xdg, "qq"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory: %w", err)
	}
	return filepath.Join(home, ".local", "state", "qq"), nil
}
