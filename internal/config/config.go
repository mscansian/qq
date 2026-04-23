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

// Config is the parsed config.toml.
type Config struct {
	History History `toml:"history"`
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

