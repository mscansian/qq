# Decision mode (`--if` / `--unless`)

`--if` and `--unless` turn `qq` into a yes/no gate for shell
pipelines. The model answers in prose as usual, and the exit code
reflects the model's verdict so you can compose with `&&`, `||`, or
`case $?`:

```
# run only if the model says yes
$ qq --if "is this log showing a real error?" < app.log && page_oncall

# open a PR only if the diff doesn't have debug prints left in it
$ git diff main | qq --unless "any debug prints or leftover console.logs?" && gh pr create
```

The prose answer still prints — you see *why* the model decided:

```
$ qq --if "is the build likely flaky?" < test.log
The failure is a timing-dependent race in the login test — the cookie
isn't set by the time the second request fires. Yes, this reads as a
flake, not a real regression.
$ echo $?
0
```

## Exit codes

| Decision | `--if` exit | `--unless` exit |
|---|---|---|
| `yes` | `0` | `1` |
| `no` | `1` | `0` |
| `unknown` | `2` | `2` |

Runtime errors never overlap with decision codes, so `qq --if "..."
&& action` is safe — it won't fire on "API was down". See
[exit-codes.md](exit-codes.md).

## When the decision doesn't parse

If the model's first line isn't a recognizable `yes` / `no` /
`unknown`, `qq` treats it as `unknown` and warns on stderr:

```
qq: model didn't follow decision format, treating as unknown
```

## When the model says `unknown`

`unknown` is the model's honest "not enough information to decide"
signal, not a parse failure. Scripts that want to escalate to a
human rather than guess can distinguish it via `case $?`:

```
qq --if "should we roll back?" < incident.log
case $? in
  0) rollback ;;
  1) : ;;                    # no, do nothing
  2) notify_human ;;         # unknown — escalate
  *) : ;;                    # runtime or config error
esac
```

## Profile system prompts

Decision mode works with custom profile system prompts. But if your
profile hard-constrains output shape (e.g. strict JSON), it will
conflict — use a separate profile for decision-mode work.

## Mutual exclusion

`--if` and `--unless` are mutually exclusive. Passing both exits
`11`.

## Safety: do not use on untrusted input

When the content reaching the model is attacker-influenced — a log
from an external source, a PR diff from a contributor, a `curl`
against a third-party URL — the attacker can embed instructions
that coerce the verdict on line 1. The prose prints afterward, but
by then `&& action` has already fired.

Use decision mode only on input you trust. For gating on untrusted
content, use a deterministic classifier. The full write-up is in
[SECURITY.md](../SECURITY.md).
