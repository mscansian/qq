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

## Control bytes

Terminal control bytes are stripped from output. See
[SECURITY.md](../SECURITY.md) for why this matters.

## Huge or binary stdin

Stdin is truncated at 200 KiB by default; raise with `--max-input`. See
[asking.md](asking.md#size-cap).
