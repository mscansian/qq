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

Every chunk streamed from the model passes through a byte filter
before hitting stdout. C0 (`0x00`–`0x1F`) and C1 (`0x80`–`0x9F`)
control bytes are stripped, with two exceptions legitimate in
plain-text output: `\n` (`0x0A`) and `\t` (`0x09`).

The reason is that model output is partially attacker-influenced
through prompt injection in stdin. Without this filter, a crafted
response could emit OSC sequences that set your terminal title,
poison your clipboard (OSC 52 on emulators that honor it), or
rewrite earlier scrollback lines via cursor addressing. The filter
operates byte-by-byte on the UTF-8 stream and is safe for multi-byte
sequences because all control bytes live in the ASCII range and
never appear inside valid UTF-8 continuation bytes.

DEL (`0x7F`), ESC, BEL, backspace, CR, and the full C1 range are
dropped.

## Large or binary stdin

- **Huge stdin**: truncated at 200 KiB by default; raise with
  `--max-input`. See [asking.md](asking.md#size-cap).
- **Binary stdin**: passed through as bytes, but likely to produce
  garbage answers — `qq` doesn't detect or reject binary input.
- **Empty stdin with no argument**: treated as a missing question;
  `qq` exits `11`.
