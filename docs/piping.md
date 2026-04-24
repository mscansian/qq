# Piping

`qq` is a Unix-style filter: reads from stdin, writes plain text to
stdout, emits diagnostics to stderr, and respects pipes at both ends.

## Reading from stdin

Stdin is auto-detected when piped:

```
$ cat error.log | qq "why is this failing?"
$ curl -s https://example.com | qq "what is this about?"
```

Pass a single `-` as the argument to force stdin usage — handy in
scripts where the source might not be a pipe:

```
$ qq - < question.txt
```

See [asking.md](asking.md) for the full matrix of arg / stdin
combinations and the size cap.

## Writing to stdout

Output is always plain text — no ANSI colors, no markdown rendering, no
TTY-conditional formatting. That makes composition with other tools
boring in the best way:

```
$ qq "list 5 common HTTP status codes, one per line" | grep 4
404 Not Found

$ qq "give me JSON with name and email for a fake user" | jq .name
"Alex Rivera"
```

## Spinner

A small spinner is shown on stderr while waiting for the first token.
Suppressed when stderr isn't a terminal (e.g. `2>log`).

## Stats

`--stats` prints a one-line summary to stderr after the response:

```
$ qq --stats "capital of France"
Paris.
qq: stats: tokens=12/3 (15 total) latency=0.82s model=gpt-5.4-mini finish=stop
```

Fields:

- `tokens=PROMPT/COMPLETION (TOTAL, CACHED cached)` — `cached=` appears
  only when the provider reported a prompt-cache hit. Shows
  `tokens=unknown` when the provider doesn't return usage.
- `latency` — wall-clock seconds from request start to stream close.
- `model` — the resolved model actually used.
- `finish` — the provider's terminal reason (`stop`, `length`,
  `content_filter`, ...). Omitted when the stream ended without one.

Stats are always on stderr, so stdout stays clean for piping. Not
printed on errors or interrupted streams — use the exit code for that.

## Control bytes

Terminal control bytes are stripped from output. See
[SECURITY.md](../SECURITY.md) for why this matters.

## Huge or binary stdin

Stdin is capped at 200 KiB by default and `qq` exits `11` above that;
raise with `--max-input`. See [asking.md](asking.md#size-cap).
