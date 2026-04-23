# Quick Question

A terminal assistant for quick questions — invoked as `qq`.

## What it is

Quick Question is a command-line tool that sends a single question to an
LLM and prints a short answer. The binary is named `qq` (for typing
ergonomics — you will reach for it dozens of times a day). It is optimized for the moment when you are already
in a terminal, you have a quick question, and opening a browser tab to a
chat UI is more friction than the question deserves.

```
$ qq "what is the archaic version of YOUR?"
The archaic version of "your" is "thy" (possessive) or "thine" before a
vowel sound — e.g. "thy sword", "thine eyes".
```

It is explicitly **not** a chatbot, not a coding agent, not a tool-using
assistant, and not multimodal. The scope is: one question in, one short
answer out.

## Target user

A developer or power user who:

- lives in the terminal and values keeping their hands on the keyboard;
- wants definitions, conversions, one-liners, quick lookups, rephrasings,
  short explanations — the kind of thing that would otherwise be a Google
  search or a flip to ChatGPT;
- already has API keys for one or more LLM providers and wants to use
  them directly without a wrapper UI.

This is not a tool for people who want long-form conversations, coding
help that spans multiple turns, or a general AI assistant experience.

## Design principles

1. **Short answers by default.** The product is the brevity. A baked-in
   system prompt instructs the model to answer in one paragraph, skip
   preamble ("Certainly!", "Great question!"), and only use multiple
   paragraphs when the question genuinely requires it.
2. **Fast perceived response.** Streaming is on by default so tokens
   appear as they arrive.
3. **Composable with the shell.** Reads from pipes, writes clean output
   when piped into other tools, uses exit codes correctly.
4. **One question, one answer.** No interactive loop, no state between
   invocations (except optional history, see below).
5. **Provider-agnostic.** Any OpenAI-compatible endpoint works — OpenAI,
   xAI (Grok), OpenRouter, DeepSeek, Groq, Ollama (local), and providers
   like Anthropic via their OpenAI-compatible endpoint.

## User flows

### Ask a question

```
$ qq "convert 180 lbs to kg"
180 pounds is approximately 81.6 kg.
```

### Pipe content in

Three input shapes are supported:

```
# argument only
$ qq "what does SIGPIPE mean?"

# stdin only (auto-detected when stdin is not a TTY)
$ cat error.log | qq

# argument + stdin — argument is the instruction, stdin is the payload
$ cat README.md | qq "summarize in 3 bullets"

# explicit stdin marker
$ qq - < question.txt
```

### Pipe output out

Output is always plain text — no ANSI colors, no markdown rendering — so
it composes cleanly with any other tool:

```
$ qq "list 5 common HTTP status codes, one per line" | grep 4
404 Not Found
```

### Choose a provider

```
$ qq -p grok "..."                    # use the 'grok' profile
$ QQ_PROFILE=local qq "..."           # via env var
$ qq -m gpt-5.4-mini "..."            # override model within profile
```

### Configure

```
$ qq --configure
```

Walks the user through creating a profile: name, base URL (pre-filled for
known providers), API key, default model. Can be run multiple times to
add additional profiles. Both config files are plain TOML and can be
edited directly for users who prefer that.

### One-off / incognito

```
$ qq --incognito "..."
```

Skips history logging for this invocation. Useful for sensitive questions
or when piping content the user doesn't want persisted.

### Decide with shell composition

`--if` and `--unless` turn `qq` into a yes/no gate for shell pipelines.
The model answers the question normally, and the exit code reflects
the decision so it composes with `&&`, `||`, or `case $?`.

```
# run only if the model says yes
$ qq --if "is this log showing a real error?" < app.log && page_oncall

# run only if the model says no
$ cat diff.patch | qq --unless "is this change risky?" && auto_merge
```

The prose answer still prints to stdout — the user sees *why* the model
decided:

```
$ qq --if "is the build likely flaky?" < test.log
The failure is a timing-dependent race in the login test — the cookie
isn't set by the time the second request fires. Yes, this reads as a
flake, not a real regression.
$ echo $?
0
```

When the model genuinely can't tell, it returns `unknown` and `qq` exits
`2`. Scripts that want to escalate to a human rather than guess can do
`case $?` to distinguish yes / no / unknown.

## Features

### Input

- Question as a positional argument.
- Question or payload from stdin (auto-detected, or explicit via `-`).
- Argument + stdin combined: argument is the instruction, stdin is the
  content being operated on.

### Output

- Streamed token-by-token to stdout as the model produces them.
- Always plain text — no ANSI colors, no markdown rendering. The product
  is short prose, not formatted documents; rendering adds complexity
  (incremental rendering is hard, full-buffer rendering loses the
  streaming feel) for little gain on this kind of answer.
- Non-zero exit code on API, network, or configuration errors, with a
  human-readable error message that says what to do.

### Decision mode

`--if <question>` and `--unless <question>` make `qq` shell-composable
as a yes/no gate. The prose answer prints to stdout as normal; the exit
code reflects the decision.

- `--if` exits `0` on **yes**, `1` on **no**.
- `--unless` exits `0` on **no**, `1` on **yes**.
- Both exit `2` on **unknown** — the model's honest "not enough
  information to decide" signal. For `&&`/`||` this behaves like a
  non-yes; for `case $?` it's distinguishable, letting scripts escalate
  to a human instead of guessing.

Runtime errors (network, API) and config errors use exit codes in the
`10+` range, so they never collide with decision codes. A `&& action`
after `qq --if` never fires on "API was down" — it only fires on a
confident yes.

### Profiles

A profile is a named set of `{base_url, api_key, model, optional
system_prompt_override}`. The profile literally named `default` is used
when no other selection is made. Selection precedence:

1. `--profile` / `-p` flag
2. `QQ_PROFILE` environment variable
3. The profile named `default` in `credentials.toml`

The `--model` / `-m` flag overrides the profile's default model for a
single invocation without requiring a new profile.

Configuration lives in two files under `~/.config/qq/` (respecting
`XDG_CONFIG_HOME`):

- `credentials.toml` — profiles and API keys. One section per profile.
  File permissions locked to `0600`. This is the sensitive file; don't
  commit it to dotfiles.
- `config.toml` — non-secret settings: default profile pointer, history
  behavior, future behavioral options. Safe to version-control.

The split mirrors the AWS CLI convention. Unlike AWS, all *per-profile*
data lives in `credentials.toml`; `config.toml` is strictly global.

### History

Every question and answer is appended to a local history file at
`~/.local/state/qq/history.jsonl` (respecting `XDG_STATE_HOME`).

- Format: JSONL, one `{timestamp, profile, model, question, answer}` per
  line.
- Rotation: capped at a fixed number of entries (e.g. 1000). When the cap
  is exceeded, oldest entries are dropped on the next write.
- Opt-out: `--incognito` skips logging for a single invocation. A
  per-profile `incognito = true` skips logging whenever that profile is
  used. A global config option disables history entirely.

History is stored for the user's own reference (re-finding an answer they
got last week) and to enable future expansions like continuation. It is
never sent back to the model as context in v1.

## Security considerations

A few properties users should be aware of before integrating `qq` into
workflows. These are the user-facing security notes — they belong in the
README too.

### Don't use decision mode on untrusted input

`--if` and `--unless` turn the model's output into a gate for shell
actions via exit codes. When stdin is attacker-influenced — a log file
from an external source, a PR diff, the body of a `curl` against a
third-party URL — the attacker can embed instructions in that content
that coerce the model to emit a chosen verdict on line 1, regardless of
the actual question being asked.

That means a pattern like:

```
$ cat diff.patch | qq --unless "is this change risky?" && auto_merge
```

is unsafe when `diff.patch` comes from a source you don't control. A
malicious diff can include prompt-injection text that steers the
response's first line to `no`, bypassing the gate. The prose answer
still prints — but by then `&& auto_merge` has already fired.

Use decision mode only on input you trust: your own logs, your own
diffs, content you've already reviewed. For gating on untrusted content,
use a deterministic classifier, not an LLM.

### Stdin and answers are persisted to history

Every non-incognito invocation appends the full question and the full
answer to `~/.local/state/qq/history.jsonl`. When the question came from
stdin — for example `cat .env | qq "which of these look like API keys?"`
— the stdin payload becomes part of the recorded question, and the
model's reply frequently echoes sensitive parts of it. Both are written
verbatim in plaintext (file mode `0600`, but plaintext).

If you pipe sensitive content (secrets, production logs, credentials,
PII) into `qq`, there are three ways to keep it out of history:

- `--incognito` on the invocation.
- `incognito = true` on a dedicated profile used for sensitive work.
- `history.enabled = false` in `config.toml` to disable history
  globally.

The default is history-on; choose the granularity that matches your
workflow.

## Non-goals

- Multi-turn conversations / interactive chat loop.
- Tool use / function calling / agentic behavior.
- Image, audio, or file attachments.
- Code editing or repo-aware features.
- Team or shared configurations.
- Cost tracking or budget enforcement.

## Proposed expansions (out of scope for v1)

These are deliberately deferred. They are each simple to add *in code*
but each one changes the product's identity — they move `qq` closer to
being a chatbot — so they need their own design treatment before being
added.

### Continuation (`--continue` / `-c`)

Resend the last question and answer as prior context along with a new
follow-up. Enables:

```
$ qq "who wrote The Left Hand of Darkness?"
Ursula K. Le Guin, published in 1969.

$ qq -c "what else did she write?"
```

This is *not* a chat loop — it is a one-shot command that happens to
include one prior exchange as context. The user invokes it explicitly
each time.

**Why it's deferred:** it raises design questions that don't have obvious
answers: how many prior turns count as "continuation" (just the last, or
a session)? What constitutes a session boundary — time, terminal, working
directory? Does the system prompt still enforce brevity when the user is
clearly building up a thread? These are product questions, not
implementation ones.

### Other possible expansions

- **`--last`**: reprint the most recent answer without re-calling the
  API.
- **`--search <query>`**: fuzzy-search history for a past answer.
- **Per-profile system prompts**: e.g. a profile named `translate` that
  always frames input as a translation task.
- **Shell completion** for profile names and flags.

## Distribution

Single static binary, distributable via:

- direct download from releases;
- `go install`;
- Homebrew (eventually).

No runtime dependencies on the user's machine — no Python, no Node, no
package manager required to run the tool.

## Language and implementation approach

Go is chosen for two reasons: single-binary distribution (no runtime for
the user to install), and a mature terminal-CLI ecosystem (TTY
detection, SSE streaming, HTTP client) that fits this tool's needs.
Any OpenAI-compatible provider is reachable with a single HTTP client —
no provider-specific SDKs are required.

Further technical detail — HTTP flow, streaming protocol, config schema,
error taxonomy — belongs in `ENGINEERING.md`, not here.
