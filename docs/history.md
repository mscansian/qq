# History

Every non-incognito invocation is logged to a local JSONL file so
you can find an answer you got last week. History is never sent
back to the model as context.

## Location

`~/.local/state/qq/history.jsonl` (honoring `XDG_STATE_HOME`).
Directory is `0700` on creation; file is `0600`.

## Record shape

One JSON object per line:

```json
{
  "timestamp": "2026-04-23T10:15:00Z",
  "profile": "openai",
  "model": "gpt-5.4-mini",
  "question": "what is the archaic version of YOUR?",
  "answer": "The archaic version of...",
  "prompt_tokens": 42,
  "completion_tokens": 17,
  "total_tokens": 59
}
```

Decision-mode invocations add a `decision` field
(`"yes"` / `"no"` / `"unknown"`). The `answer` field always stores
the prose only, matching what you saw on stdout.

`payload_bytes` is set only on the arg+stdin shape (`cat foo | qq
"…"`), where the recorded `question` is just the arg and the actual
content lived in stdin. It records how much was piped in. Stdin-only
invocations leave it at zero — their payload is already in
`question`.

## Reusing the last answer

When a follow-up needs the previous turn for context — e.g. the model
misread the question and you want to clarify — `qq --last` prints the
most recent question and answer in a form you can paste back as
context:

```
$ qq --last
Previous question: what's an idempotent HTTP method?
Previous answer: One whose effect is the same whether called once
or many times — GET, PUT, DELETE.

$ qq "$(qq --last)

but what about PATCH?"
```

`--last` doesn't call the model and doesn't write to history. If the
previous turn was `cat foo | qq "…"` the actual stdin payload isn't
in history, so the output gets a marker:

```
Previous question: what's wrong?
(previous turn included ~3.2 KiB of piped content, not shown)
Previous answer: ...
```

so you know the model's answer was based on data you'd need to
re-pipe to truly reuse.

## Rotation

The file is capped at `history.max_entries` (default `1000`, set in
[`config.toml`](config.md#configtoml)). Once over the cap, the next
append trims it back to the most recent `max_entries` lines.

## Opt out

Three ways to keep content out of history, from narrowest to
broadest:

1. **`--incognito`** on a single invocation:
   ```
   $ qq --incognito "paraphrase this: ..."
   ```
2. **Per-profile** `incognito = true` in `credentials.toml`.
   Handy for a profile reserved for sensitive work:
   ```toml
   [work]
   # ...
   incognito = true
   ```
   Every `qq -p work ...` is then skipped automatically.
3. **Global off** in `config.toml`:
   ```toml
   [history]
   enabled = false
   ```

## Partial responses

Partial answers are saved on interruption. A failure to write
history never fails the invocation — `qq` prints a one-line warning
to stderr and returns the answer's exit code.

## Security: history captures what you piped in

When a question is constructed from stdin, the entire stdin
payload becomes part of the recorded question. The answer often
echoes sensitive bits back. Both are plaintext — file mode is
`0600` but the contents are readable by anything that can read
the file.

If you pipe secrets, production logs, credentials, or PII into
`qq`, use one of the opt-outs above. See [SECURITY.md](../SECURITY.md)
for the full write-up.
