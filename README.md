# qq — Quick Question

A tiny terminal assistant for quick questions. Type `qq`, ask, get an
answer. No chat UI, no browser tab, no conversation — just one question in
and a short answer out.

Works with any OpenAI-compatible provider (OpenAI, xAI, Anthropic, OpenRouter,
Groq, DeepSeek, Ollama). You just need to configure your API key.

```
$ qq "how do I undo the last commit but keep my changes"
Run `git reset --soft HEAD~1` — it rewinds the commit but leaves
your changes staged, ready to re-commit.

$ qq explain SIGKILL
SIGKILL is an uncatchable, unignorable Unix signal used to immediately
terminate a process; it can't be handled or cleaned up by the program,
so the OS stops it right away.
```

## Why you'd want it

You already live in the terminal. Most of the stuff you'd type into
ChatGPT is a one-liner: a unit conversion, a word you can't remember, a
`grep` flag, a quick rephrase. Opening a browser tab for that is more
friction than the question deserves.

`qq` answers in one paragraph, streams the response as it's produced, and
composes cleanly with pipes and shell operators.

## Things you can do with it

**Ask anything**

```
$ qq "curl flag for following redirects"
Use -L (or --location); curl doesn't follow redirects by default.
```

**Interpret input**

```
$ go test ./... 2>&1 | qq "which test is the real failure?"

$ git diff --staged | qq "give me a one-line commit header"

$ curl -s https://example.com/page | qq "what is this about?"
```

**Generate output** — output is always plain text, so it redirects
and composes cleanly:

```
$ qq "list 5 common HTTP status codes, one per line" | grep 4
404 Not Found

$ qq "a .gitignore for a Python + Node project" > .gitignore
```

**Decide in shell scripts** — `--if` and `--unless` turn the model's
answer into an exit code so you can wire it up with `&&` and `||`:

```
# page only if the model thinks the log is a real error
$ cat app.log | qq --if "is this log showing a real error?" && page_oncall

# commit only if the diff doesn't sneak in a public-API change
$ git diff --staged | qq --unless "does this touch the public API?" && git commit
```

Exit `0` = yes, `1` = no, `2` = unknown — the prose still prints. Full
contract in [`docs/decision-mode.md`](docs/decision-mode.md). See [`SECURITY.md`](SECURITY.md) before piping untrusted input.

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

- [Asking a question](docs/asking.md) — input shapes, quoting, the
  system prompt.
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
piping secrets, credentials, production data, or untrusted input into
it. The short version:


## License

MIT — see [`LICENSE.md`](LICENSE.md).

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for project layout, build
instructions, and where to start if you want to add a feature.
