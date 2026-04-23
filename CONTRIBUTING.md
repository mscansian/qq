# Contributing to qq

Thanks for taking a look. `qq` tries to be a Unix tool in the old
sense: small, composable, does one thing. That thing is *ask a
question, get a short answer.* Contributions that sharpen that use
case are welcome; contributions that turn it into something else are
not.

## Philosophy

Think `grep`, `jq`, `curl` — tools you reach for without thinking,
that compose with pipes, that stay out of your way. `qq` aims for
that bar. A few consequences:

- **One question in, one answer out.** No interactive loop, no
  conversation, no session state beyond optional history.
- **Plain text.** Stdout is the answer; stderr is for diagnostics and
  the spinner. No markdown rendering, no colors, no TTY-conditional
  formatting. Output that pipes cleanly into `grep`, `awk`, or a file.
- **Stays out of the way.** Fast to type, fast to start streaming, no
  prompts you have to dismiss, no config wizards you have to re-run.
- **Provider-agnostic.** Any OpenAI-compatible endpoint works.

The sharpest way to think about scope: if a feature pushes `qq`
toward *conversation*, *agency*, or *content processing beyond a short
answer*, it's probably out. Open an issue to discuss before coding so
you don't burn time on something that won't merge.

## Security stance

`qq` lives in your shell, not a sandbox. That shapes how we think
about safety: we take care not to hand attackers a free channel
through piped content or model output, but we also don't block your
choices behind prompts or "are you sure?" gates. The bar is simple —
**`qq` should never do something you didn't ask for.**

Concretely, a few things in the codebase exist for this reason and
shouldn't be weakened without a clear reason in the PR:

- **Untrusted data is framed as data.** Stdin combined with an
  argument gets wrapped in `<content>...</content>` tags, with the
  closing tag neutralized inside the payload. The system prompt
  treats anything inside those tags as data, not instructions.
- **The model can't poke the terminal.** Streamed output passes
  through a control-byte filter that strips C0/C1 bytes (keeping
  `\n` / `\t`), so a crafted response can't set the terminal title,
  hit clipboard-via-OSC, or rewrite earlier lines.
- **Exit codes stay honest.** Decision mode (`--if` / `--unless`)
  uses `0`/`1`/`2` for yes/no/unknown; runtime and config errors
  start at `10`. That separation is why `qq --if "..." && action`
  doesn't fire on "API was down".

These are bar-raisers, not guarantees. Prompt injection isn't fully
solvable at the prompt level — decision mode on attacker-controlled
stdin is still unsafe, and [`SECURITY.md`](SECURITY.md) documents
that honestly rather than pretending otherwise.

## Requirements

- Go 1.22 or newer (developed against Go 1.25).
- No Node, no Python, no Docker.

## Getting the code

```
$ git clone https://github.com/mscansian/qq.git
$ cd qq
$ go build ./cmd/qq
$ ./qq --help
```

## Running tests

```
$ go test ./...
$ go vet ./...
```

## Building locally

```
$ go build -o qq ./cmd/qq
```


## How to add a feature

1. **Re-read "Philosophy" and "Security stance" above.** If your
   feature pushes against either, open an issue before coding.
2. **Keep the scope tight.** Only change what your feature needs.
   Unrelated refactors and "while I'm here" cleanups belong in
   separate PRs.
3. **Update the docs you touched.** If you add a flag, update
   `README.md` and this file.

## AI-assisted PRs

Vibe-coded / AI-assisted PRs are fine — I use the same tools. What's
expected:

- **Read your own PR before opening it.** If you haven't, I'll be able
  to tell.
- **Make sure it actually works.** Tests pass, the feature does what
  the description claims, the diff doesn't include unrelated churn
  the model threw in.
- **Write the description yourself.** A human-readable "what and why"
  in a few sentences. Not a model-generated wall of bullet points.

PRs that look like they were submitted without the author engaging —
broken tests, scope creep, generated noise, changes that clearly
don't fit the philosophy above — will be closed without detailed
feedback. The bar isn't "no AI"; it's "a human stood behind this".

## Reporting bugs

- Bugs and feature requests go on GitHub Issues.
- Security issues go through the private advisory flow documented in
  [`SECURITY.md`](SECURITY.md) — not the public issue tracker.

## License

By contributing, you agree that your contribution will be licensed
under the MIT License (see [`LICENSE.md`](LICENSE.md)).
