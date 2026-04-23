# Providers

`qq` talks to any OpenAI-compatible endpoint. `--configure`
pre-fills the base URL and a default model for the common ones;
anything else goes through the `Custom` option.

## Supported out of the box

| Provider | Base URL | Default model | Notes |
|---|---|---|---|
| OpenAI | `https://api.openai.com/v1` | `gpt-5.4-mini` | |
| xAI / Grok | `https://api.x.ai/v1` | `grok-4-1-fast-non-reasoning` | |
| Anthropic | `https://api.anthropic.com/v1` | `claude-haiku-4-5` | experimental |
| OpenRouter | `https://openrouter.ai/api/v1` | `x-ai/grok-4.1-fast` | experimental |
| Groq | `https://api.groq.com/openai/v1` | `llama-3.1-8b-instant` | experimental |
| DeepSeek | `https://api.deepseek.com` | `deepseek-chat` | experimental |
| Ollama (local) | `http://localhost:11434/v1` | `llama3.2` | experimental, runs offline |
| Custom | you provide it | you provide it | any OpenAI-compatible endpoint |

**"Experimental"** means the provider's OpenAI-compatible mode
works but hasn't been as thoroughly exercised as OpenAI / xAI.
Report breakage.

## Model choice

The default picks target the cheap / fast tier on purpose. `qq`'s
job is short answers, not reasoning — reasoning models are slower,
more expensive, and their "thinking" passes fight with the brevity
system prompt.

Override the model per invocation with `-m`:

```
$ qq -m gpt-5.4-mini "..."
```

Or change the profile default by running `qq --configure` again.

## The OpenAI-compatible assumption

`qq` uses the **Chat Completions API** (`/v1/chat/completions`),
not the Responses API. Responses is OpenAI-specific; Chat
Completions is the lingua franca every other provider settled on.
That's why any OpenAI-compatible endpoint works with one client.

Provider-specific features — Anthropic's native API, OpenAI's
Responses API, provider-side tool calling — are out of scope by
design.

## Custom endpoints

The `Custom` option in `--configure` just prompts for a base URL.
Anything that speaks Chat Completions over TLS will work:

- Self-hosted: Ollama, LM Studio, vLLM, TGI.
- Internal gateways / proxies (e.g. a per-team LLM gateway).
- Staging or regional endpoints for a known provider.

`qq` does **not** validate the base URL at configure time — no
pre-flight request. If a provider changes a URL or a response field
that breaks compatibility, the failure surfaces on first use with a
runtime error, not during setup.

## Security: `base_url` is a credential-theft vector

`base_url` is accepted verbatim from the config file or
`QQ_BASE_URL`. Pointing `qq` at an attacker-controlled endpoint
walks your API key out the door on the first invocation — over the
connection the attacker chose, in the Authorization header the
attacker now holds.

If you paste a config snippet from outside (tutorial, gist, Slack
message), inspect the `base_url` the same way you'd inspect a
shell one-liner before pasting it. Prefer HTTPS; the only
legitimate `http://` is a local model on loopback
(e.g. `http://localhost:11434/v1` for Ollama).

See [SECURITY.md](../SECURITY.md) for the full write-up.
