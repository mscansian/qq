# Exit codes

The exit-code table is **universal**: codes `1` and `2` are always
reserved for decision-mode semantics, even when `--if` / `--unless`
isn't active. In normal mode those codes are simply never emitted
— tool errors skip straight to `10+`. That keeps the contract
stable across invocation modes, so a script piping `qq` through
`&&` behaves predictably whether the flag is present or not.

| Code | Meaning | When emitted |
|---|---|---|
| `0` | Success (normal) / **yes** (`--if`) / **no** (`--unless`) | Happy path |
| `1` | **no** (`--if`) / **yes** (`--unless`) | Decision mode only — never emitted in normal mode |
| `2` | **unknown** — model couldn't decide, or its first line didn't parse as yes/no/unknown | Decision mode only — never emitted in normal mode |
| `10` | Runtime error | Network timeout, API 5xx, API 4xx (after sanitization), rate-limited after retry |
| `11` | Usage / config error | Bad flags, conflicting flags, missing required field, no profile configured, bad TOML |
| `130` | Interrupted | Ctrl-C (SIGINT), or SIGTERM |

Decision-mode semantics are detailed in
[decision-mode.md](decision-mode.md).

## Error message format

Errors follow a `qq: <what failed>: <why>` shape and include
remediation when possible. A few examples:

```
qq: no default profile found. Run 'qq --configure' or set QQ_API_KEY, QQ_BASE_URL, QQ_MODEL.
qq: profile 'grok' not found in credentials.toml.
qq: provider returned 401 Unauthorized. Check the API key in profile 'default'.
qq: --if and --unless are mutually exclusive.
```

Provider error bodies are passed through but stripped of
`Authorization`-adjacent noise. API keys never appear in any
output.

## Timeouts, retries, cancellation

- **Per-request timeout**: 120 seconds. Long enough for slow
  reasoning-capable models; short enough that a hung provider
  doesn't wedge the CLI.
- **Retries**: handled by the SDK's default policy (429 / 5xx).
  No retry on timeout — the timeout already bounds total wall time.
- **Cancellation**: SIGINT or SIGTERM aborts the request, flushes
  partial output, and exits `130`.

## Designing scripts around this

The codes let a script distinguish "the model said no", "the
model couldn't decide", and "something is broken" without
parsing stdout:

```
qq --if "should we roll back?" < incident.log
case $? in
  0)   rollback ;;           # yes
  1)   : ;;                  # no
  2)   notify_human ;;       # unknown — escalate
  10)  retry_later ;;        # runtime error
  11)  abort_bad_config ;;   # usage / config error
  130) : ;;                  # interrupted
esac
```

Because `10+` never collides with decision codes, the common
shortcut is safe:

```
$ qq --if "..." && action
```

This runs `action` only on a confident yes — never on `unknown`,
never on "API was down", never on a config error.
