package config

import (
	"fmt"
	"os"
	"time"
)

// Resolved is the fully-resolved runtime config for a single invocation.
type Resolved struct {
	ProfileName  string // name of the selected profile, or "" when fully driven by env vars
	BaseURL      string
	APIKey       string
	Model        string
	SystemPrompt string        // empty means "use the baked-in default"
	Incognito    bool          // from profile.incognito
	Timeout      time.Duration // zero means "fall through to config.toml then default"
	MaxBytes     int           // zero means "fall through to config.toml then default"
	OnOverflow   string        // empty means "fall through to config.toml then default"
}

// Overrides are the command-line flags / env vars that override profile
// fields. Zero values mean "no override".
type Overrides struct {
	Profile string // --profile / -p
	Model   string // --model / -m
}

// Resolve applies the precedence ladder documented in ENGINEERING.md:
//
//  1. --profile → QQ_PROFILE → profile named "default"
//  2. --model → QQ_MODEL → profile.model
//  3. QQ_API_KEY → profile.api_key
//  4. QQ_BASE_URL → profile.base_url
//
// system_prompt and incognito come from the selected profile only.
//
// If no profile is available AND QQ_* env vars don't fully cover the three
// required fields, an error is returned naming the first missing field.
func Resolve(creds *Credentials, ov Overrides) (*Resolved, error) {
	res := &Resolved{}

	// 1. Select profile (if any).
	name := ov.Profile
	if name == "" {
		name = os.Getenv("QQ_PROFILE")
	}

	var prof Profile
	var havePro bool
	if name != "" {
		p, ok := creds.Profiles[name]
		if !ok {
			return nil, fmt.Errorf("profile %q not found in %s", name, creds.Path)
		}
		prof = p
		havePro = true
		res.ProfileName = name
	} else if p, ok := creds.Profiles["default"]; ok {
		prof = p
		havePro = true
		res.ProfileName = "default"
	}

	// 2. Model: --model > QQ_MODEL > profile.
	res.Model = firstNonEmpty(ov.Model, os.Getenv("QQ_MODEL"), prof.Model)

	// 3 & 4. Key and base URL: QQ_* env > profile.
	res.APIKey = firstNonEmpty(os.Getenv("QQ_API_KEY"), prof.APIKey)
	res.BaseURL = firstNonEmpty(os.Getenv("QQ_BASE_URL"), prof.BaseURL)

	// Profile-only fields.
	if havePro {
		res.SystemPrompt = prof.SystemPrompt
		res.Incognito = prof.Incognito
		if prof.Timeout != "" {
			d, err := time.ParseDuration(prof.Timeout)
			if err != nil {
				return nil, fmt.Errorf("profile %q: timeout %q is not a Go duration; use e.g. \"45s\" or \"3m\"", res.ProfileName, prof.Timeout)
			}
			if d <= 0 {
				return nil, fmt.Errorf("profile %q: timeout must be positive, got %q", res.ProfileName, prof.Timeout)
			}
			res.Timeout = d
		}
		if prof.MaxBytes < 0 {
			return nil, fmt.Errorf("profile %q: max_bytes must be positive, got %d", res.ProfileName, prof.MaxBytes)
		}
		res.MaxBytes = prof.MaxBytes
		switch prof.OnOverflow {
		case "", OnOverflowError, OnOverflowTruncate:
			res.OnOverflow = prof.OnOverflow
		default:
			return nil, fmt.Errorf("profile %q: on_overflow must be %q or %q, got %q", res.ProfileName, OnOverflowError, OnOverflowTruncate, prof.OnOverflow)
		}
	}

	// Validate required fields.
	if res.APIKey == "" {
		return nil, missingFieldError("api_key", name, havePro, creds)
	}
	if res.BaseURL == "" {
		return nil, missingFieldError("base_url", name, havePro, creds)
	}
	if res.Model == "" {
		return nil, missingFieldError("model", name, havePro, creds)
	}
	return res, nil
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

func missingFieldError(field, name string, havePro bool, creds *Credentials) error {
	if !havePro && name == "" {
		return fmt.Errorf(
			"no default profile found. Run 'qq --configure' or set QQ_API_KEY, QQ_BASE_URL, QQ_MODEL (missing: %s)",
			field,
		)
	}
	if !havePro {
		return fmt.Errorf("profile %q not found in %s (missing: %s)", name, creds.Path, field)
	}
	return fmt.Errorf("profile %q is missing %s", name, field)
}
