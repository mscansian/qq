# Asking a question

`qq` takes a question three ways:

```
# argument only
$ qq "what does SIGPIPE mean?"

# stdin only (auto-detected when stdin is piped)
$ cat error.log | qq

# argument + stdin — argument is the instruction, stdin is the payload
$ cat README.md | qq "summarize in 3 bullets"

# explicit stdin marker
$ qq - < question.txt
```

## Quoting

Quotes are only needed when the question contains shell metacharacters
(`?`, `*`, `|`, `&`, `!`, `$`, backticks). Plain sentences work unquoted:

```
$ qq explain race conditions
```

## Argument rules

| Situation | Behavior |
|---|---|
| Arg present | that's the question (multiple args are treated as one question) |
| No arg, stdin is piped | stdin is the question |
| No arg, stdin is a TTY | usage error, exits `11` |
| Arg + piped stdin | arg is the instruction, stdin is the payload |
| Single `-` as arg | force stdin even if it's a TTY |

## Arg + stdin

When both are provided, the argument is the instruction and stdin is the
content being operated on. Stdin is framed as untrusted data before being
sent to the model — see [SECURITY.md](../SECURITY.md) for what that does
and doesn't cover.

## Size cap

Stdin is read up to **200 KiB** by default. Above that, `qq` truncates,
prints a one-line warning to stderr, and proceeds with the truncated
payload. Raise the cap with `--max-input=BYTES` for one invocation, or
set `input.max_bytes` in `config.toml` to change the default globally.
