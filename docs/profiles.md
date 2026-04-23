# Profiles

A profile is a named `{base_url, api_key, model}` combo, optionally
with a per-profile system prompt or incognito flag. Profiles let you
switch providers, models, or contexts without editing configs.

## Interactive setup

```
$ qq --configure
```

Walks through creating or editing a profile:

1. **Profile name.** Blank → `default`.
2. **Provider.** Pick from a pre-filled list (see
   [providers.md](providers.md)) or `Custom` for a manual base URL.
   The pick only pre-fills `base_url` and a default model suggestion
   — no provider field is stored.
3. **API key.** Echo is disabled during input. Stored literally in
   the file.
4. **Default model.** Prefilled based on provider choice; editable.
5. **Overwrite confirmation.** If the profile already exists, `qq`
   shows the current values (API key redacted) alongside the new
   ones and asks to confirm. Default is no; declining aborts without
   writing.
6. **Default promotion.** If this is the first profile and you named
   it something other than `default`, `qq` offers to rename it to
   `default` so the resolver falls back to it when no `-p` or
   `QQ_PROFILE` is set.

Before writing, `qq` previews the TOML block that will be added and
asks to confirm. Run `qq --configure` again any time to add or
update profiles.

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
| `base_url` | string | yes | OpenAI-compatible endpoint root (must include `/v1` or equivalent) |
| `api_key` | string | yes | API key, stored literally |
| `model` | string | yes | Model identifier passed verbatim to the provider |
| `system_prompt` | string | no | Replaces the default system prompt for this profile |
| `incognito` | bool | no | When `true`, invocations using this profile skip history |

Unknown fields are rejected — a typo in a key name is a loud error,
not a silent fall-through to defaults.

## Per-profile system prompts

`system_prompt` **replaces** the baked-in prompt wholesale — it does
not append. That lets a task-specific profile (`translate`, `grep`,
`lint`) set its own framing without the brevity guidance fighting
with it.

Two consequences to know about:

- **Injection resistance is yours to keep.** The baked-in prompt
  frames `<content>...</content>` as untrusted data. An override
  prompt loses that framing unless you reinstate it. See
  [asking.md](asking.md#the-system-prompt).
- **Decision mode still composes.** `--if` / `--unless` append a
  format-enforcement block after your `system_prompt`, preserving
  your framing. But if your prompt hard-constrains output shape
  (e.g. strict JSON), it will conflict. See
  [decision-mode.md](decision-mode.md#known-incompatibility).

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

## File layout

- `~/.config/qq/credentials.toml` — profiles and API keys.
  Sensitive.
- `~/.config/qq/config.toml` — global non-secret settings. See
  [config.md](config.md).

The split mirrors the AWS CLI convention: sensitive per-profile
data in one file, global behavior in another. Unlike AWS, all
per-profile data lives in `credentials.toml`; `config.toml` is
strictly global.

Directory mode is `0700` on creation.

## Security notes

- API keys are stored in plaintext on disk; protect the file the
  same way you'd protect any other API-key file.
- Don't commit `credentials.toml` to a dotfiles repo, even a
  private one.
- For CI or remote hosts, prefer the `QQ_*` env vars from a secret
  manager over copying the file.
- A hostile `base_url` can steal your API key on the first request.
  Inspect any `base_url` value you didn't type yourself.

See [SECURITY.md](../SECURITY.md) for the detailed threat model.
