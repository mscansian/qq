# qq — Quick Question

A tiny terminal assistant for quick questions. Type `qq`, ask, get an
answer. No chat UI, no browser tab, no conversation — just one question in
and a short answer out.

```
$ qq "what is the archaic version of YOUR?"
The archaic version of "your" is "thy" (possessive) or "thine" before a
vowel sound — e.g. "thy sword", "thine eyes".
```

Quotes are only needed when the question contains shell metacharacters
(`?`, `*`, `|`, `&`, `!`, `$`, backticks). Plain sentences work
unquoted: `qq explain SIGKILL`.

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
$ qq "convert 180 lbs to kg"
180 pounds is approximately 81.6 kg.
```

**Pipe content in**

```
$ cat README.md | qq "summarize in 3 bullets"
$ curl -s https://example.com/page | qq "what is this about?"
$ cat error.log | qq "why is this failing?"
```

**Pipe content out** — output is always plain text, so `grep`, `awk`,
`jq` all work:

```
$ qq "list 5 common HTTP status codes, one per line" | grep 4
404 Not Found
```

**Use it as a yes/no gate in shell scripts** — `--if` and `--unless`
turn the model's answer into an exit code so you can wire it up with
`&&` and `||`:

```
# page only if the model thinks the log is a real error
$ qq --if "is this log showing a real error?" < app.log && page_oncall

# auto-merge only if the model is confident the diff is safe
$ cat diff.patch | qq --unless "is this change risky?" && auto_merge
```

The prose answer still prints, so you see *why* the model decided.
Exit `0` = yes, `1` = no, `2` = unknown. (See the
[security notes](SECURITY.md) before pointing this at untrusted input.)

**Keep a private invocation out of history**

```
$ qq --incognito "paraphrase this: ..."
```

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

`qq` is a single static binary — no Python, no Node, no package manager
needed to run it.

## Configure

Run the interactive setup once:

```
$ qq --configure
```

It asks for a profile name, a provider, an API key, and a default model.
You can run it again any time to add another profile or update an
existing one. Profile config lives in `~/.config/qq/credentials.toml`
(mode `0600`) and is plain TOML if you'd rather edit it by hand.

### Multiple profiles

A profile is a saved `{provider, model, API key}` combo. You can switch
between them per-invocation:

```
$ qq -p grok "..."             # use the 'grok' profile
$ QQ_PROFILE=local qq "..."    # via env var
$ qq -m gpt-5.4-mini "..."     # override just the model
```

When no profile is specified, `qq` uses the one literally named
`default`.

### No config file at all

You can skip the config file entirely and drive `qq` with environment
variables — handy for CI or one-off scripts:

```
$ export QQ_API_KEY=sk-...
$ export QQ_BASE_URL=https://api.openai.com/v1
$ export QQ_MODEL=gpt-5.4-mini
$ qq "..."
```

## Supported providers

Any OpenAI-compatible endpoint works. `qq --configure` pre-fills the
base URL and a sensible default model for the ones below:

| Provider | Base URL | Default model suggestion | Notes |
|---|---|---|---|
| OpenAI | `https://api.openai.com/v1` | `gpt-5.4-mini` | |
| xAI / Grok | `https://api.x.ai/v1` | `grok-4-1-fast-non-reasoning` | |
| Anthropic | `https://api.anthropic.com/v1` | `claude-haiku-4-5` | experimental |
| OpenRouter | `https://openrouter.ai/api/v1` | `x-ai/grok-4.1-fast` | experimental |
| Groq | `https://api.groq.com/openai/v1` | `llama-3.1-8b-instant` | experimental |
| DeepSeek | `https://api.deepseek.com` | `deepseek-chat` | experimental |
| Ollama (local) | `http://localhost:11434/v1` | `llama3.2` | experimental, runs offline |
| Custom | you provide it | you provide it | anything OpenAI-compatible |

The default picks target the cheap/fast tier — `qq`'s job is short
answers, not reasoning. You can override the model per-invocation with
`-m`, or change the profile default by re-running `qq --configure`.

## History

Every non-incognito invocation is logged to
`~/.local/state/qq/history.jsonl` (one JSON record per line — question,
answer, timestamp, model). The file caps at 1000 entries and rotates
oldest-first.

Ways to keep content out of history:

- `--incognito` on a single invocation.
- Set `incognito = true` on a profile you reserve for sensitive work;
  then `qq -p work` is automatically incognito.
- Disable it globally by setting `history.enabled = false` in
  `~/.config/qq/config.toml`.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | success (or "yes" in `--if`, "no" in `--unless`) |
| 1 | "no" in `--if`, "yes" in `--unless` |
| 2 | unknown — model couldn't decide |
| 10 | runtime error (network, provider 5xx, timeout) |
| 11 | usage / config error (bad flags, missing profile) |
| 130 | interrupted (Ctrl-C) |

Codes `1` and `2` are reserved for decision mode even outside it — so
`qq "..." && next` and `qq --if "..." && next` behave the same way when
something goes wrong.

## Security

`qq` sends whatever you ask to a third-party provider and logs the
exchange to disk by default. Read [`SECURITY.md`](SECURITY.md) before
piping secrets, credentials, production data, or untrusted input into
it. The short version:

- **Don't** use `--if` / `--unless` on content you didn't write — the
  model's verdict is controllable by whoever wrote the content.
- **Do** use `--incognito` (or an incognito profile) when the question
  involves sensitive material.
- **Do** protect `~/.config/qq/credentials.toml` like any other API-key
  file.

## License

MIT — see [`LICENSE.md`](LICENSE.md).

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for project layout, build
instructions, and where to start if you want to add a feature.
