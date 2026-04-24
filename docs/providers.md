# Providers

`qq` talks to any OpenAI-compatible endpoint. `--configure` pre-fills
the base URL and a default model for the common ones; anything else
goes through the `Custom` option.

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

**"Experimental"** means the provider's OpenAI-compatible mode works
but hasn't been as thoroughly exercised as OpenAI / xAI. Report
breakage.

## Overriding the model

```
$ qq -m gpt-5.4-mini "..."
```

Or change the profile default by running `qq --configure` again.

## Custom endpoints

The `Custom` option in `--configure` prompts for a base URL. Typical
uses:

- Self-hosted: Ollama, LM Studio, vLLM, TGI.
- Internal gateways / proxies (e.g. a per-team LLM gateway).
- Staging or regional endpoints for a known provider.

## Bedrock, Vertex, Azure OpenAI

These providers don't speak OpenAI-compat directly — Bedrock uses
SigV4, Vertex uses Google auth, Azure OpenAI differs in path shape
and headers. Run a translation proxy locally or in your infra and
point `qq` at it via the `Custom` provider:

- [LiteLLM](https://github.com/BerriAI/litellm) fronts Bedrock,
  Vertex, Azure, and many other providers with an OpenAI-compatible
  API. It reads `~/.aws/credentials`, Google ADC, etc. on its side,
  so `qq` only sees the proxy URL.
- AWS ships a Bedrock Access Gateway sample that does the same thing
  Bedrock-only.

## Security

A hostile `base_url` walks your API key out the door on the first
invocation. Inspect `base_url` values from tutorials, gists, or Slack
before pasting them. See [SECURITY.md](../SECURITY.md).
