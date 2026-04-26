# Exit codes

Codes `1` and `2` are reserved for decision-mode semantics and are
never emitted in normal mode — tool errors skip straight to `10+`.

| Code | Meaning | When emitted |
|---|---|---|
| `0` | Success (normal) / **yes** (`--if`) / **no** (`--unless`) | Happy path |
| `1` | **no** (`--if`) / **yes** (`--unless`) | Decision mode only — never emitted in normal mode |
| `2` | **unknown** — model couldn't decide, or its first line didn't parse as yes/no/unknown | Decision mode only — never emitted in normal mode |
| `10` | Runtime error | Network timeout, API 5xx, API 4xx, rate-limited after retry |
| `11` | Usage / config error | Bad flags, conflicting flags, missing required field, no profile configured, bad TOML |
| `130` | Interrupted | Ctrl-C (SIGINT), SIGTERM, or `--interactive` declined at the prompt |

Decision-mode semantics are detailed in
[decision-mode.md](decision-mode.md).

## Error messages

Errors follow a `qq: <what failed>: <why>` shape on stderr and
include remediation when possible. API keys never appear in any
output. See [troubleshooting.md](troubleshooting.md) for the common
ones.

## Timeouts and cancellation

Per-request timeout is 120 seconds. Ctrl-C aborts cleanly and exits
`130`.

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
