# Configuration

Non-secret behavior settings, environment variables, and how the
layers compose. For per-profile credentials, see
[profiles.md](profiles.md).

## Files

Both config files live under `~/.config/qq/` (honoring
`XDG_CONFIG_HOME`). The directory is created with mode `0700` on
first `--configure` run.

| File | Mode | What's in it |
|---|---|---|
| `credentials.toml` | `0600` | Profiles and API keys. Sensitive. See [profiles.md](profiles.md). |
| `config.toml` | `0644` | Global non-secret settings. Strictly global, never per-profile. |

The split mirrors the AWS CLI convention. Unlike AWS, all
per-profile data lives in `credentials.toml`; `config.toml` is
strictly global.

## `config.toml`

```toml
[history]
enabled = true
max_entries = 1000
```

### Fields

| Field | Type | Default | Purpose |
|---|---|---|---|
| `history.enabled` | bool | `true` | Whether to append Q&A to `history.jsonl`. |
| `history.max_entries` | int | `1000` | Rotation cap. See [history.md](history.md). |

Unknown fields are rejected — a typo in a key name is a loud
error, not a silent fall-through to defaults.

## Environment variables

| Variable | Purpose |
|---|---|
| `QQ_PROFILE` | Select a profile by name. Same effect as `-p`. |
| `QQ_API_KEY` | Override `api_key` for this invocation. |
| `QQ_BASE_URL` | Override `base_url` for this invocation. |
| `QQ_MODEL` | Override `model` for this invocation. Same effect as `-m`. |
| `XDG_CONFIG_HOME` | Override config directory. Default `~/.config`. |
| `XDG_STATE_HOME` | Override state directory. Default `~/.local/state`. |

### Env-var-only mode

Set all three of `QQ_API_KEY`, `QQ_BASE_URL`, `QQ_MODEL` and `qq`
runs with no config file at all:

```
$ export QQ_API_KEY=sk-...
$ export QQ_BASE_URL=https://api.openai.com/v1
$ export QQ_MODEL=gpt-5.4-mini
$ qq "..."
```

Useful for CI, containers, or one-off scripts where writing a
credentials file is overkill.

## Resolution order

Each field is resolved through this ladder, highest-priority-first.
Missing layers fall through.

**Profile selection:**

1. `--profile` / `-p` flag
2. `QQ_PROFILE` env var
3. The profile literally named `default` in `credentials.toml`

**Within a profile, per field:**

1. Flag (model only: `--model` / `-m`)
2. `QQ_API_KEY` / `QQ_BASE_URL` / `QQ_MODEL` env vars
3. The selected profile's field

If a required field (`api_key`, `base_url`, `model`) can't be
resolved through any layer, `qq` exits `11` with a config error
naming the specific missing field.
