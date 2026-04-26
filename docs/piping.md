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

## Confirm before piping (`--interactive`, `-i`)

Pass `-i` to preview the response on the terminal's alternate screen
(same trick `less` and `vim` use) and require confirmation before
anything is written to stdout. Useful when the next stage in the
pipeline runs the output (e.g. `| sh`) and you want a human checkpoint.
After confirming, the alt screen is restored, so the preview leaves no
trace in your scrollback.

```
$ ls | qq -i "ffmpeg command to extract a frame every 10s into frames/" | sh

ffmpeg -i input.mp4 -vf fps=1/10 frames/frame_%03d.png

Pipe to next command? [y/N]
```

Default is no — only `y` or `yes` (any case) accepts and flushes the
response to stdout. Anything else, including a bare Enter, aborts with
exit `130` (same class as Ctrl-C) and stdout stays empty, so the
downstream command receives nothing.

`-i` requires a terminal: stdout must be piped or redirected and
`/dev/tty` must be openable. It can't be combined with `--if`/`--unless`.

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
