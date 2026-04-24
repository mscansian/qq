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

## Stdin passthrough

In decision mode, `qq` behaves like a `grep`-style filter on stdin:

- On a gate-open verdict (exit 0), the original stdin is written
  verbatim to stdout.
- On any other verdict (`no`/`yes` depending on mode, or `unknown`),
  stdout stays empty. The exit code carries the answer.
- The model's prose goes to **stderr** so stdout stays reserved for
  passthrough.

This makes `qq` chainable with the next stage of a pipeline:

```
# only hand the script to sh if the model doesn't flag it
$ curl -fsSL https://install.example/setup.sh \
    | qq --unless "does this look malicious?" \
    | sh

# only commit the diff if no debug prints sneaked in
$ git diff --staged | qq --unless "any debug prints?" | git apply --cached
```

The prose answer prints to stderr — you see *why* the model decided
even while stdout flows to the next command:

```
$ qq --if "is the build likely flaky?" < test.log 2>reason
$ cat reason
The failure is a timing-dependent race in the login test — the cookie
isn't set by the time the second request fires. Yes, this reads as a
flake, not a real regression.
$ echo $?
0
```

Arg-only decision mode (no stdin) still works — stdout just stays
empty because there's nothing to pass through:

```
$ qq --if "is water wet?" && echo yes
yes
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

Stdout stays empty (unknown never opens the gate).

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

## The verdict is a lean, not an attestation

The model is instructed to commit to `yes` or `no` when the
evidence leans that way, even when it isn't fully certain. Any
remaining uncertainty lives in the prose. That makes the exit code
useful for composition (`&& action`) but also makes it a lossy
summary — a confident verdict and a leaning one look the same to
the shell.

For safety-flavored questions ("does this look malicious?", "any
secrets in this diff?") read the prose before acting. For gates that
actually need to be correct, use a deterministic classifier. The
full write-up is in [SECURITY.md](../SECURITY.md#3-the-verdict-is-a-lossy-summary-of-the-models-judgment).

## Reproducibility

Decision mode pins the request temperature to `0`, so repeated
runs on the same input with the same model return the same
verdict. How strong that reproducibility is depends on the
provider. Different models or providers can still disagree on
borderline inputs — reproducibility is within-model, not
across-model.

Normal mode (no `--if`/`--unless`) keeps the provider's default
temperature, so generation queries like `qq "a .gitignore for X"`
stay re-rollable.

## Truncation and passthrough

By default `qq` refuses oversize stdin (exits `11`) so the model
never judges a prefix you didn't notice was clipped. If you set
`input.on_overflow = truncate`, the model sees the prefix *and* the
passthrough carries only that prefix — the downstream command gets
a short read.

## Safety: do not use on untrusted input

When the content reaching the model is attacker-influenced — a log
from an external source, a PR diff from a contributor, a `curl`
against a third-party URL — the attacker can embed instructions
that coerce the verdict on line 1. The downstream command in the
pipe then sees the original payload exactly as if the gate had
legitimately opened.

Use decision mode only on input you trust. For gating on untrusted
content, use a deterministic classifier. The full write-up is in
[SECURITY.md](../SECURITY.md).
