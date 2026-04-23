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
  "answer": "The archaic version of..."
}
```

Under decision mode, an extra field is included:

```json
{
  "timestamp": "2026-04-23T10:15:00Z",
  "profile": "openai",
  "model": "gpt-5.4-mini",
  "question": "is this log showing a real error?",
  "answer": "The failure is a timing-dependent race...",
  "decision": "yes"
}
```

`answer` always stores the prose portion only, never the raw
decision line — matching what you saw on stdout. See
[decision-mode.md](decision-mode.md).

Only fields not derivable from the request context are recorded:
no API key, no base URL, no latency metrics.

## Rotation

The file is capped at `history.max_entries` (default `1000`, set in
[`config.toml`](config.md#configtoml)). On each append, if the
line count exceeds the cap, `qq` rewrites the file keeping only
the last `max_entries - 1` lines plus the new one.

Rotation is done in-place rather than via a temp file — acceptable
because history is not durability-critical.

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

History is written after the stream ends. If it ends cleanly, the
complete answer is saved. If it ends abnormally — SIGINT, network
error, API error mid-stream — whatever was received is saved with
the question. No special marker is added; the truncation is
implicit and matches what you saw on stdout.

A failure to write history never fails the invocation. `qq` prints
a one-line warning to stderr and returns the answer's exit code:

```
qq: warning: failed to write history: <reason>
```

## Concurrency

There is no file locking. With `O_APPEND` on POSIX and record
sizes well under `PIPE_BUF` (4 KB), individual appends are atomic
and lines won't interleave within themselves. Rotation (full
rewrite) is the only non-atomic operation, and it's rare — every
Nth invocation, once the cap is reached. The tradeoff is
occasional lost rotation if two processes rotate simultaneously,
which is acceptable for a personal-use tool.

## Security: history captures what you piped in

When a question is constructed from stdin, the entire stdin
payload becomes part of the recorded question. The answer often
echoes sensitive bits back. Both are plaintext — file mode is
`0600` but the contents are readable by anything that can read
the file.

If you pipe secrets, production logs, credentials, or PII into
`qq`, use one of the opt-outs above. See [SECURITY.md](../SECURITY.md)
for the full write-up.
