# Troubleshooting

Common failures and how to recognize them. Error messages are
prefixed with `qq: ` on stderr; see
[exit-codes.md](exit-codes.md) for the exit-code table.

## `qq: no default profile found`

You ran `qq` without `-p` / `QQ_PROFILE`, and there's no profile
literally named `default` in `credentials.toml`. Two fixes:

- Run `qq --configure` and create a profile named `default` (or
  promote the first profile you create when prompted).
- Use env-var-only mode: set `QQ_API_KEY`, `QQ_BASE_URL`,
  `QQ_MODEL` and run `qq` without a config file. See
  [config.md](config.md#env-var-only-mode).

Exits `11`.

## `qq: profile 'X' not found in credentials.toml`

Typo in `-p` or `QQ_PROFILE`, or the profile hasn't been created
yet. `qq --configure` lists and edits profiles.

Exits `11`.

## `qq: provider returned 401 Unauthorized`

The API key in the active profile is wrong, revoked, or doesn't
belong to the `base_url` it's being sent to. Fix by re-running
`qq --configure` for that profile, or by setting `QQ_API_KEY` for
a one-off.

Exits `10`.

## `qq: --if and --unless are mutually exclusive`

Only one decision-mode flag at a time. See
[decision-mode.md](decision-mode.md).

Exits `11`.

## `qq: stdin exceeds N bytes; raise --max-input or set input.on_overflow = "truncate"`

Your stdin exceeded the cap (200 KiB by default, or whatever
`input.max_bytes` is set to in `config.toml`). `qq` refuses the
call rather than judge a prefix. Raise the cap with
`--max-input=BYTES` (raw byte count, no suffixes):

```
$ cat big.log | qq --max-input=1048576 "summarize"
```

If you'd rather accept a prefix, set
`input.on_overflow = "truncate"` in `config.toml`.

Exits `11`.

## `qq: stdin truncated at N bytes; use --max-input to override`

You set `input.on_overflow = "truncate"` in `config.toml` and
stdin exceeded the cap. `qq` proceeds with the truncated payload;
the answer may be incomplete. Raise the cap with
`--max-input=BYTES` (raw byte count, no suffixes), or revert to
the default `input.on_overflow = "error"` to refuse oversize input.

Not fatal — exits `0` if the request succeeds.

## `qq: model didn't follow decision format, treating as unknown`

In decision mode, the model's first line wasn't a recognizable
`yes` / `no` / `unknown` token. `qq` treats it as `unknown` and
exits `2`. The prose still streams to stdout so you can see what
the model actually said.

Common causes:

- The active profile has a `system_prompt` that hard-constrains
  output shape (e.g. strict JSON). That's incompatible with
  decision mode — use a separate profile. See
  [decision-mode.md](decision-mode.md#profile-system-prompts).
- A weaker / local model that doesn't reliably follow the format
  instruction. Try a stronger model with `-m`.

## `qq: warning: credentials.toml is readable by others`

File permissions on `credentials.toml` drifted. Re-chmod:

```
$ chmod 0600 ~/.config/qq/credentials.toml
```

Consider rotating the affected key if you share the host.

## `qq: warning: failed to write history`

History writes are non-fatal. `qq` returns the answer's exit code
regardless. Likely causes: disk full, permission drift on
`~/.local/state/qq/`, or the file being held open by another tool.

To stop the warnings entirely, set `history.enabled = false` in
`config.toml`. See [history.md](history.md).

## The request hangs / times out

Per-request timeout is 60 seconds by default. A hung provider
surfaces as a runtime error (exit `10`) after that point. Check
network connectivity and the provider's status page. SDK retries
happen automatically on 429 / 5xx but not on timeout.

Raise it for slow models with `--timeout 5m`, or set a per-profile
or global default — see [config.md](config.md#fields).

Interrupt manually with Ctrl-C — `qq` flushes partial output and
exits `130`.

## Stdin isn't being picked up

`qq` auto-detects stdin by checking if it's a TTY. If your script
source might not be a pipe (e.g. redirecting from a file in a
subshell in some environments), pass `-` to make stdin usage
explicit:

```
$ qq - < question.txt
```

## History file is growing unexpectedly

History caps at `history.max_entries` (default `1000`) and
rotates oldest-first. If the file is much larger than that, you
may have multiple `qq` processes racing on rotation — acceptable
in practice but occasionally costs a rotation. Either truncate
the file manually, or disable history globally:

```toml
# ~/.config/qq/config.toml
[history]
enabled = false
```

See [history.md](history.md).
