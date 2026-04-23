package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
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

	stream := client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model: req.Model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(req.SystemPrompt),
			openai.UserMessage(req.UserMessage),
		},
	})

	proc := newProcessor(stdout, stderr, req.Decision)

	for stream.Next() {
		evt := stream.Current()
		if len(evt.Choices) == 0 {
			continue
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

	resp := &Response{Prose: proc.Prose()}
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
		return errors.New("request timed out")
	}
	return err
}
