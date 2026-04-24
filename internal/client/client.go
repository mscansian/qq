package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
)

// Request is a single LLM invocation.
type Request struct {
	BaseURL      string
	APIKey       string
	Model        string
	SystemPrompt string // already composed via ComposeSystemPrompt
	UserMessage  string
	// Decision toggles the output state machine. The system prompt must
	// have been composed with decision=true to match.
	Decision bool
}

// Response is what Run returns after the stream completes.
type Response struct {
	// Prose is the text that was written to stdout (minus the decision
	// line), suitable for recording in history.
	Prose string
	// Decision is set iff Request.Decision was true.
	Decision Decision
	// Usage carries token counts from the provider. UsageKnown is false
	// when the stream ended before the final usage chunk arrived
	// (interrupted stream, provider that doesn't support include_usage).
	Usage      Usage
	UsageKnown bool
	// FinishReason is the terminal reason reported on the last choice
	// chunk ("stop", "length", "content_filter", ...). Empty when the
	// provider didn't supply one (interrupted stream, unusual provider).
	FinishReason string
}

// Usage is the token-count breakdown for one request. CachedTokens is a
// subset of PromptTokens that the provider served from its prompt cache,
// zero when not reported.
type Usage struct {
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
	CachedTokens     int64
}

// Run opens an SSE stream to the configured endpoint and pipes deltas
// through the control-byte filter and (if enabled) the decision state
// machine into stdout. Errors written to stderr are status-only; the
// prose itself goes to stdout.
func Run(ctx context.Context, req Request, stdout, stderr io.Writer) (*Response, error) {
	client := openai.NewClient(
		option.WithAPIKey(req.APIKey),
		option.WithBaseURL(normalizeBaseURL(req.BaseURL)),
	)

	params := openai.ChatCompletionNewParams{
		Model: req.Model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(req.SystemPrompt),
			openai.UserMessage(req.UserMessage),
		},
		StreamOptions: openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: param.NewOpt(true),
		},
	}
	if req.Decision {
		// Pin temperature to 0 in decision mode for reproducibility —
		// the 1-bit verdict is already a lossy summary, sampling
		// variance on top turns identical inputs into different exit
		// codes. Normal mode keeps provider defaults so generation
		// queries ("a .gitignore for X") stay re-rollable.
		params.Temperature = param.NewOpt(0.0)
	}
	stream := client.Chat.Completions.NewStreaming(ctx, params)

	proc := newProcessor(stdout, stderr, req.Decision)

	var usage Usage
	var usageKnown bool
	var finishReason string

	for stream.Next() {
		evt := stream.Current()
		// The final chunk carries token usage (and no choices) when
		// include_usage is enabled. Some providers may omit it; we leave
		// usageKnown false in that case.
		if evt.Usage.TotalTokens > 0 {
			usage = Usage{
				PromptTokens:     evt.Usage.PromptTokens,
				CompletionTokens: evt.Usage.CompletionTokens,
				TotalTokens:      evt.Usage.TotalTokens,
				CachedTokens:     evt.Usage.PromptTokensDetails.CachedTokens,
			}
			usageKnown = true
		}
		if len(evt.Choices) == 0 {
			continue
		}
		if r := evt.Choices[0].FinishReason; r != "" {
			finishReason = r
		}
		content := evt.Choices[0].Delta.Content
		if content == "" {
			continue
		}
		filtered := stripControlBytes([]byte(content))
		if len(filtered) == 0 {
			continue
		}
		if _, err := proc.Write(filtered); err != nil {
			return nil, fmt.Errorf("write output: %w", err)
		}
	}

	if err := stream.Err(); err != nil {
		// Close the output cleanly so the user sees whatever we got.
		_ = proc.Close()
		return nil, mapError(err)
	}

	if err := proc.Close(); err != nil {
		return nil, fmt.Errorf("finalize output: %w", err)
	}

	resp := &Response{
		Prose:        proc.Prose(),
		Usage:        usage,
		UsageKnown:   usageKnown,
		FinishReason: finishReason,
	}
	if d, ok := proc.Decision(); ok {
		resp.Decision = d
	}
	return resp, nil
}

// normalizeBaseURL ensures the base URL ends with "/" so openai-go's
// relative-path resolution (RFC 3986) extends rather than replaces the
// last segment. Without this, "https://api.x.ai/v1" + "chat/completions"
// resolves to "https://api.x.ai/chat/completions" and 404s.
func normalizeBaseURL(u string) string {
	if u == "" || strings.HasSuffix(u, "/") {
		return u
	}
	return u + "/"
}

// mapError converts openai-go / transport errors into a qq-shaped error
// that the CLI layer can render with the `qq: <what>: <why>` format.
func mapError(err error) error {
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		// Many providers (xAI among them) return a 404 with an empty
		// body when the path or model is unknown, so apiErr.Message is
		// blank. Include the request URL so the user can diagnose.
		// We don't %w here: the apiErr's own Error() string repeats the
		// status and message we just extracted, so wrapping would
		// double the text in the user-visible output.
		msg := apiErr.Message
		if msg == "" && apiErr.Request != nil && apiErr.Request.URL != nil {
			msg = "(empty body) URL=" + apiErr.Request.URL.String()
		}
		return fmt.Errorf("provider returned %d: %s", apiErr.StatusCode, msg)
	}
	if errors.Is(err, context.Canceled) {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("request timed out: %w", err)
	}
	return err
}
