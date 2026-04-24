# qq — the LLM as a Unix tool

Your shell already has `jq` for JSON, `grep` for patterns, `curl` for
HTTP. `qq` is the LLM primitive: pipe text in, get text out, exit
`0`/`1` for yes/no. One API call per invocation. No session, no chat
UI, no filesystem access — it can't read your code or run commands,
and doesn't want to.

Works with any OpenAI-compatible provider (OpenAI, xAI, Anthropic,
OpenRouter, Groq, DeepSeek, Ollama). Bring your own API key.

```
$ git diff --staged | qq "one-line commit header"
Fix off-by-one in pagination bounds

$ cat app.log | qq --if "real error?" && page_oncall

$ qq "curl flag for following redirects"
Use -L (or --location); curl doesn't follow redirects by default.
```

## What it's for

`qq` treats the model as a text transform with an optional question.
Pipe a diff in, get a commit header. Pipe logs in, ask which test
failed. Pipe anything in, get back a yes/no exit code.

Typing `qq "…"` to ask a one-off question works too, but the real
job is composing with pipes, redirects, and `&&`/`||`.

## How is this different from coding agents?

Claude Code, opencode, aider are the top-level process — you start
one and it runs the show. `qq` is a tool the shell calls. That's why
it fits inside scripts, pipes, cron jobs, and git hooks, and they
don't.

## Shapes

**Ask** — one question, one answer.

```
$ qq explain SIGKILL
SIGKILL is an uncatchable, unignorable Unix signal used to immediately
terminate a process; it can't be handled or cleaned up by the program,
so the OS stops it right away.
```

**Pipe** — stdin goes to the model alongside your question. Output is
plain text, so it flows into whatever comes next:

```
$ go test ./... 2>&1 | qq "which test is the real failure?"

$ curl -s https://example.com/page | qq "what is this about?"

$ qq "list 5 common HTTP status codes, one per line" | grep 4
404 Not Found

$ qq "a .gitignore for a Python + Node project" > .gitignore
```

**Script** — `--if` and `--unless` turn the answer into an exit code,
so `qq` wires up with `&&` and `||` like any other shell tool:

```
# page only if the model thinks the log is a real error
$ cat app.log | qq --if "is this log showing a real error?" && page_oncall

# commit only if the diff doesn't sneak in a public-API change
$ git diff --staged | qq --unless "does this touch the public API?" && git commit
```

Exit `0` = yes, `1` = no, `2` = unknown — the prose still prints.
Full contract in [`docs/decision-mode.md`](docs/decision-mode.md).
See [`SECURITY.md`](SECURITY.md) before piping untrusted input.

## Install

### From source

```
$ go install github.com/mscansian/qq/cmd/qq@latest
```

That puts a `qq` binary in `$(go env GOPATH)/bin`. Make sure that's on
your `PATH`.

### Download a release

Grab a pre-built binary for your platform from the
[releases page](https://github.com/mscansian/qq/releases) and drop it
anywhere on your `PATH`.

```
$ curl -L https://github.com/mscansian/qq/releases/latest/download/qq-linux-amd64 -o /usr/local/bin/qq
$ chmod +x /usr/local/bin/qq
```

Available builds: `qq-linux-amd64`, `qq-linux-arm64`,
`qq-darwin-amd64` (Intel Mac), `qq-darwin-arm64` (Apple Silicon).

Before your first question, you'll need a provider API key — see
[Configure](#configure) below.

## Configure

Run the interactive setup once:

```
$ qq --configure
```

It asks for a profile name, a provider, an API key, and a default model.
Run it again any time to add another profile or update an existing one.

Need a key? [OpenAI](https://platform.openai.com/api-keys) ·
[xAI](https://console.x.ai) (API Keys section). For other providers,
see [`docs/providers.md`](docs/providers.md).

Switch profiles per-invocation, or skip the config file entirely:

```
$ qq -p grok "..."             # use the 'grok' profile
$ QQ_PROFILE=local qq "..."    # via env var
$ qq -m gpt-5.4-mini "..."     # override just the model

# env-var-only mode, no config file
$ QQ_API_KEY=... QQ_BASE_URL=... QQ_MODEL=... qq "..."
```

Any OpenAI-compatible endpoint works, including local ones like
Ollama or a custom URL.

## Documentation

- [`docs/`](docs/README.md) — how `qq` behaves, flags, config, and
  caveats.
- [`SECURITY.md`](SECURITY.md) — threat model and what to avoid
  piping in.
- [`CONTRIBUTING.md`](CONTRIBUTING.md) — build, test, project layout.

Highlights:

- [Asking a question](docs/asking.md) — input shapes and quoting.
- [Profiles](docs/profiles.md) and [configuration](docs/config.md) —
  precedence, env vars, file layout.
- [Decision mode](docs/decision-mode.md) — `--if` / `--unless` and
  the yes/no/unknown contract.
- [History](docs/history.md) — record shape, rotation, opt-outs.
- [Exit codes](docs/exit-codes.md) — full table and script
  patterns.
- [Troubleshooting](docs/troubleshooting.md) — common failures.

## Security

`qq` sends whatever you ask to a third-party provider and logs the
exchange to disk by default. Read [`SECURITY.md`](SECURITY.md) before
piping secrets, credentials, production data, or untrusted input
into it.

## License

MIT — see [`LICENSE.md`](LICENSE.md).

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for project layout, build
instructions, and where to start if you want to add a feature.
