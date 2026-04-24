# qq documentation

Detailed notes on how `qq` behaves, what it accepts, and the caveats
you'll hit along the way. The top-level [README](../README.md) covers
the happy path; this is the reference.

## Usage

- [Asking a question](asking.md) — input shapes, when to quote, how
  args and stdin combine, the baked-in system prompt.
- [Piping](piping.md) — stdin detection, shell composition, the size
  cap, the control-byte filter on output.
- [Decision mode](decision-mode.md) — `--if` and `--unless`, the
  yes/no/unknown contract, known incompatibilities.
- [Exit codes](exit-codes.md) — the full table, why `1`/`2` are
  reserved universally, error-message conventions.

## Setup

- [Profiles](profiles.md) — `credentials.toml`, `--configure`,
  precedence between flag / env / default profile.
- [Providers](providers.md) — supported endpoints, default models,
  the OpenAI-compatible assumption.
- [Configuration](config.md) — `config.toml`, `QQ_*` env vars, how
  fields resolve across layers.

## Operations

- [History](history.md) — record shape, rotation, three ways to opt
  out.
- [Troubleshooting](troubleshooting.md) — common failures and how to
  recognize them.

## Not covered here

- [SECURITY.md](../SECURITY.md) — threat-model topics: prompt
  injection, decision mode on untrusted input, credential file
  protection, `base_url` as a credential-theft vector.
- [CONTRIBUTING.md](../CONTRIBUTING.md) — build, test, and project
  layout for contributors.
