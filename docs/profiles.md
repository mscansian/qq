# Profiles

A profile is a named `{base_url, api_key, model}` combo, optionally
with a per-profile system prompt or incognito flag. Profiles let you
switch providers, models, or contexts without editing configs.

## Interactive setup

```
$ qq --configure
```

Prompts for a profile name, provider, API key, and default model,
then writes to `credentials.toml`. Run it again any time to add or
edit profiles.

## Selecting a profile

Highest-priority-first:

1. `--profile` / `-p` flag — `qq -p grok "..."`
2. `QQ_PROFILE` environment variable — `QQ_PROFILE=local qq "..."`
3. The profile literally named `default` in `credentials.toml`

If none of those resolve to an existing profile, `qq` exits `11`
with a config error naming what's missing.

## Overriding the model

Use `-m` / `--model` to swap the model for a single invocation
without editing the profile:

```
$ qq -m gpt-5.4-mini "..."
```

Full field-level resolution order, highest-priority-first:

1. `--model` flag (model only)
2. `QQ_API_KEY`, `QQ_BASE_URL`, `QQ_MODEL` env vars — narrow escape
   hatches for CI and one-offs. See [config.md](config.md).
3. The selected profile's fields.

If a required field (`api_key`, `base_url`, `model`) can't be
resolved through any layer, `qq` exits `11` with a config error
naming the specific missing field.

## `credentials.toml`

Lives at `~/.config/qq/credentials.toml` (honoring
`XDG_CONFIG_HOME`). One TOML section per profile; the section name
is the profile name. File mode is locked to `0600`; `qq` warns on
stderr if it drifts so that other users on the machine could read
it.

```toml
[default]
base_url = "https://api.openai.com/v1"
api_key = "sk-..."
model = "gpt-5.4-mini"

[grok]
base_url = "https://api.x.ai/v1"
api_key = "xai-..."
model = "grok-4-1-fast-non-reasoning"

[local]
base_url = "http://localhost:11434/v1"
api_key = "ollama"
model = "llama3.2"

[work]
base_url = "https://api.openai.com/v1"
api_key = "sk-..."
model = "gpt-5.4-mini"
incognito = true

[translate]
base_url = "https://api.openai.com/v1"
api_key = "sk-..."
model = "gpt-5.4-mini"
system_prompt = "Translate the user's input to English. Output only the translation."
```

### Fields

| Field | Type | Required | Purpose |
|---|---|---|---|
| `base_url` | string | yes | OpenAI-compatible endpoint root |
| `api_key` | string | yes | API key |
| `model` | string | yes | Model identifier |
| `system_prompt` | string | no | Replaces the default system prompt for this profile |
| `incognito` | bool | no | When `true`, invocations using this profile skip history |
| `timeout` | string | no | Per-request timeout (Go duration, e.g. `"45s"`, `"3m"`). Overrides `request.timeout` in [config.md](config.md); overridden by `--timeout`. |

Unknown fields are rejected.

## Per-profile system prompts

`system_prompt` **replaces** the default prompt wholesale. If it
hard-constrains output shape (e.g. strict JSON), it will conflict
with decision mode — see
[decision-mode.md](decision-mode.md#profile-system-prompts).

The per-invocation `<content-NONCE>...</content-NONCE>`
untrusted-data block is appended after your prompt whenever stdin
is wrapped (arg + stdin shape), regardless of whether the default
prompt or an override is in use — so injection-resistance stays
on when you customize the prompt. See
[SECURITY.md](../SECURITY.md).

## Per-profile incognito

Setting `incognito = true` on a profile skips history for every
invocation that uses it. Useful when a profile is reserved for
sensitive work:

```
$ qq -p work "..."   # never written to history
```

This composes with the `--incognito` flag and the global kill
switch in `config.toml`. See [history.md](history.md) for all
three.

## Security notes

- API keys are stored in plaintext on disk; protect the file the
  same way you'd protect any other API-key file.
- Don't commit `credentials.toml` to a dotfiles repo, even a
  private one.
- For CI or remote hosts, prefer the `QQ_*` env vars from a secret
  manager over copying the file.

See [SECURITY.md](../SECURITY.md) for the detailed threat model.
