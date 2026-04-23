# Contributing to qq

Thanks for taking a look. `qq` is intentionally small: one question in,
one short answer out. Contributions that keep it small are very welcome;
contributions that turn it into a chatbot or an agent are not
(see the "Non-goals" section in `DESIGN.md`).

## Overview

`qq` is a Go CLI that sends a single chat completion to an
OpenAI-compatible endpoint and streams the response to stdout. The
design is captured in two documents:

- [`DESIGN.md`](DESIGN.md) — product-level description: what it is,
  who it's for, what the UX looks like.
- [`ENGINEERING.md`](ENGINEERING.md) — technical plan: language,
  dependencies, layout, error codes, streaming, history.
- [`SECURITY.md`](SECURITY.md) — threat model and known risks.

Read those first if you're planning anything non-trivial — they
explain *why* the code is the way it is, and a fair amount of what
looks like complexity (`<content>` wrapping, control-byte filter,
decision-mode state machine) is load-bearing for a specific threat or
usability property.

## Requirements

- Go 1.22 or newer (developed against Go 1.25).
- That's it. No Node, no Python, no Docker.

## Getting the code

```
$ git clone https://github.com/mscansian/qq.git
$ cd qq
$ go build ./cmd/qq
$ ./qq --help
```

## Project layout

```
cmd/qq/
  main.go                 entrypoint — just calls cli.Execute
internal/
  cli/
    root.go               cobra root command, flag wiring, exit code mapping
    configure.go          interactive --configure flow
    spinner.go            one-character TTY spinner
  config/
    credentials.go        credentials.toml loader + save
    config.go             config.toml loader + XDG paths
    resolve.go            profile resolution ladder (flags → env → profile)
  client/
    client.go             openai-go wrapper, SSE streaming loop
    systemprompt.go       baked-in system prompt + decision format block
    filter.go             UTF-8-aware control-byte stripper
    decision.go           decision-mode output state machine
  history/
    history.go            JSONL append + rotation
  input/
    input.go              stdin detection, arg+stdin combination, size cap
```

Each package has a single responsibility and is unit-testable without
the others. `cli` depends on everything; everything else avoids
depending on `cli`.

## Running tests

```
$ go test ./...
```

All tests are unit or in-process integration — there are no tests
against real providers. The client layer uses `httptest.Server` to
impersonate an OpenAI-compatible SSE stream, which covers request
shape, streaming, control-byte filtering, and decision parsing.

```
$ go vet ./...
```

## Building locally

```
$ go build -o qq ./cmd/qq
```

Or inject a version string:

```
$ go build -ldflags "-X github.com/mscansian/qq/internal/cli.Version=$(git describe --tags --always)" -o qq ./cmd/qq
```

## How to add a feature

1. **Read `DESIGN.md` and `ENGINEERING.md`.** If your feature changes
   the product identity (multi-turn, tool use, image input), it
   probably belongs under "Proposed expansions" rather than being
   merged. Open an issue to discuss before coding.
2. **Find the right package.** Most features naturally fit one of the
   existing packages. Resist the urge to create a new one for a single
   function.
3. **Keep it testable.** Prefer table-driven tests (`map[string]struct`)
   and behavioral tests over mock-heavy ones. If you need to inject
   an interface, define it at the consumer's boundary, not upfront
   everywhere.
4. **Keep the scope tight.** Only change what your feature needs.
   Unrelated refactors and "while I'm here" cleanups make PRs harder
   to review — ship them separately.
5. **Update the docs you touched.** If you add a flag, update
   `README.md` and the CLI surface table in `ENGINEERING.md`.

## Code style

This repo follows the Go style guide at
[`go-style`](https://github.com/matheus-consoli/claude-settings) in
broad strokes: short names in small scopes, table-driven tests, no
unnecessary interfaces, godoc comments only when they add information
beyond the signature. If you're deviating from an existing pattern,
please explain why in the PR description.

## Commit messages

- One-line header, imperative voice, ≤72 characters.
- A body is optional; use it to explain *why*, not *what*. Good
  candidates: trade-offs, constraints, the bug this fixes, why you
  chose this approach over an alternative.
- Squash fixups into the commit they fix before opening the PR.

## Reporting bugs

- Bugs and feature requests go on GitHub Issues.
- Security issues go through the private advisory flow documented in
  [`SECURITY.md`](SECURITY.md) — not the public issue tracker.

## License

By contributing, you agree that your contribution will be licensed
under the MIT License (see [`LICENSE.md`](LICENSE.md)).
