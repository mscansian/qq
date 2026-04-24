# Piping

`qq` is a Unix-style filter: it reads from stdin, writes plain text
to stdout, emits diagnostics to stderr, and respects pipes at both
ends.

## Reading from stdin

`qq` auto-detects stdin by checking whether it's a TTY. If it's not,
stdin is read as input:

```
$ cat error.log | qq "why is this failing?"
$ curl -s https://example.com | qq "what is this about?"
```

Pass a single `-` as the argument to make stdin usage explicit —
handy in scripts where the source might not be a pipe:

```
$ qq - < question.txt
```

See [asking.md](asking.md) for the full matrix of arg / stdin
combinations and the size cap.

## Writing to stdout

Output is always plain text — no ANSI colors, no markdown rendering,
no TTY-conditional formatting. That makes composition with other
tools boring in the best way:

```
$ qq "list 5 common HTTP status codes, one per line" | grep 4
404 Not Found

$ qq "give me JSON with name and email for a fake user" | jq .name
"Alex Rivera"
```

Tokens stream to stdout unbuffered as the model produces them. `qq`
ensures the final output ends with a newline even if the model
didn't produce one.

## The spinner

When stderr is a TTY, `qq` prints a small spinner to stderr while
waiting for the first token. It clears as soon as streaming starts.

When stderr is **not** a TTY (e.g. `qq "..." 2>log`), no spinner is
written — so log files don't fill up with spinner glyphs. The check
is specifically on stderr's fd, not stdout's, so piping stdout into
another tool still shows the spinner on your terminal.

## Control-byte filtering

Streamed output is filtered to remove terminal control bytes —
everything in the C0 range (`0x00`–`0x1F`) except `\n` and `\t`,
plus DEL (`0x7F`) and stray C1 bytes (`0x80`–`0x9F`). The filter
is UTF-8-aware, so valid multi-byte runes pass through intact.

This exists because model output can be steered by prompt
injection in stdin. Without it, a crafted answer could emit
escape sequences that set your terminal title, trigger OSC 52
clipboard writes, or rewrite earlier scrollback lines.

## Large or binary stdin

- **Huge stdin**: truncated at 200 KiB by default; raise with
  `--max-input`. See [asking.md](asking.md#size-cap).
- **Binary stdin**: passed through as bytes, but likely to produce
  garbage answers — `qq` doesn't detect or reject binary input.
- **Empty stdin with no argument**: treated as a missing question;
  `qq` exits `11`.
