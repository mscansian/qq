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

Tool errors (`10`, `11`, `130`) are mode-independent — they never
collide with `0` / `1` / `2`. That's why `qq --if "..." && action`
doesn't fire on "API was down": a network error exits `10`, which
`&&` treats as failure.

See [exit-codes.md](exit-codes.md) for the universal table.

## How the decision is extracted

The decision-mode system prompt asks the model to put exactly one
word — `yes`, `no`, or `unknown` — on the first line, a blank line,
then the prose explanation. `qq` parses the stream like this:

1. Buffer deltas until the first `\n`. Nothing is printed to stdout
   yet; the spinner stays visible.
2. Lowercase the buffered line and extract the first `[a-z]+`
   token. So `Yes,` → `yes`, `YES!` → `yes`,
   `yes — because...` → `yes`.
3. Match against `yes` / `no` / `unknown`. Anything else (including
   an empty line) is treated as `unknown`, and a one-line warning
   goes to stderr:
   ```
   qq: model didn't follow decision format, treating as unknown
   ```
4. Skip one blank separator line. If the model omits it, `qq`
   tolerates the drift and streams anyway.
5. Stream the remaining deltas to stdout.

If the stream ends before any newline arrives, the whole response is
treated as the decision attempt and parsed the same way — likely
producing `unknown`.

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

## Composition with profile system prompts

Decision mode **composes** with whatever base prompt is already
active — it does not replace it. A format-enforcement block is
appended to the profile's `system_prompt` (or the baked-in default),
so role and domain context are preserved.

### Known incompatibility

Profiles whose `system_prompt` hard-constrains output shape — e.g.
"always respond in strict JSON" — will conflict with the decision
format. That combination isn't supported. If you need both, use
separate profiles.

## Mutual exclusion

`--if` and `--unless` are mutually exclusive. Passing both exits
`11`.

## Interaction with history

Decision-mode invocations get an extra `decision` field
(`"yes"` / `"no"` / `"unknown"`) in the history record. The `answer`
field stores the prose portion only, stripped of the decision line —
matching what you saw on stdout. See [history.md](history.md).

## Safety: do not use on untrusted input

When the content reaching the model is attacker-influenced — a log
from an external source, a PR diff from a contributor, a `curl`
against a third-party URL — the attacker can embed instructions
that coerce the verdict on line 1. The prose prints afterward, but
by then `&& action` has already fired.

Use decision mode only on input you trust. For gating on untrusted
content, use a deterministic classifier. The full write-up is in
[SECURITY.md](../SECURITY.md).
