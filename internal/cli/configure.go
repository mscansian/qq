package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"golang.org/x/term"

	"github.com/mscansian/qq/internal/config"
)

// provider is one option on the configure menu. Only base_url and the
// default model suggestion are stored from the selection; no provider
// field is persisted.
type provider struct {
	label    string
	baseURL  string
	model    string
	tag      string // empty or "(experimental)"
	isCustom bool
}

var providers = []provider{
	{label: "OpenAI", baseURL: "https://api.openai.com/v1", model: "gpt-5.4-mini"},
	{label: "xAI / Grok", baseURL: "https://api.x.ai/v1", model: "grok-4-1-fast-non-reasoning"},
	{label: "Anthropic", baseURL: "https://api.anthropic.com/v1", model: "claude-haiku-4-5", tag: "(experimental)"},
	{label: "OpenRouter", baseURL: "https://openrouter.ai/api/v1", model: "x-ai/grok-4.1-fast", tag: "(experimental)"},
	{label: "Groq", baseURL: "https://api.groq.com/openai/v1", model: "llama-3.1-8b-instant", tag: "(experimental)"},
	{label: "DeepSeek", baseURL: "https://api.deepseek.com", model: "deepseek-chat", tag: "(experimental)"},
	{label: "Ollama (local)", baseURL: "http://localhost:11434/v1", model: "llama3.2", tag: "(experimental)"},
	{label: "Custom", isCustom: true},
}

func runConfigure(stdin io.Reader, stdout, _ io.Writer) error {
	br := bufio.NewReader(stdin)

	// 1. Profile name.
	fmt.Fprint(stdout, "Profile name [default]: ")
	name, err := readLine(br)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "default"
	}

	// 2. Provider menu.
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "Select a provider:")
	for i, p := range providers {
		tag := ""
		if p.tag != "" {
			tag = " " + p.tag
		}
		fmt.Fprintf(stdout, "  %d) %s%s\n", i+1, p.label, tag)
	}
	fmt.Fprint(stdout, "Choice [1]: ")
	choiceRaw, err := readLine(br)
	if err != nil {
		return err
	}
	choice := 1
	if strings.TrimSpace(choiceRaw) != "" {
		if _, err := fmt.Sscanf(choiceRaw, "%d", &choice); err != nil {
			return usageErrorf("invalid choice %q", choiceRaw)
		}
	}
	if choice < 1 || choice > len(providers) {
		return usageErrorf("choice out of range")
	}
	p := providers[choice-1]

	baseURL := p.baseURL
	if p.isCustom {
		fmt.Fprint(stdout, "Base URL: ")
		u, err := readLine(br)
		if err != nil {
			return err
		}
		baseURL = strings.TrimSpace(u)
		if baseURL == "" {
			return usageErrorf("base URL cannot be empty")
		}
	}

	// 3. API key (no echo if stdin is a TTY).
	fmt.Fprintln(stdout)
	apiKey, err := readSecret(stdin, stdout, "API key: ")
	if err != nil {
		return err
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return usageErrorf("API key cannot be empty")
	}

	// 4. Default model.
	prompt := "Default model"
	if p.model != "" {
		prompt += fmt.Sprintf(" [%s]", p.model)
	}
	fmt.Fprintf(stdout, "%s: ", prompt)
	modelRaw, err := readLine(br)
	if err != nil {
		return err
	}
	model := strings.TrimSpace(modelRaw)
	if model == "" {
		model = p.model
	}
	if model == "" {
		return usageErrorf("model cannot be empty")
	}

	// 5. Overwrite check.
	creds, err := config.LoadCredentials()
	if err != nil {
		return usageErrorf("%s", err)
	}
	if existing, ok := creds.Profiles[name]; ok {
		fmt.Fprintln(stdout)
		fmt.Fprintf(stdout, "Profile %q already exists:\n", name)
		fmt.Fprintf(stdout, "  base_url: %s\n", existing.BaseURL)
		fmt.Fprintf(stdout, "  api_key:  %s\n", redact(existing.APIKey))
		fmt.Fprintf(stdout, "  model:    %s\n", existing.Model)
		fmt.Fprintln(stdout, "New values:")
		fmt.Fprintf(stdout, "  base_url: %s\n", baseURL)
		fmt.Fprintf(stdout, "  api_key:  %s\n", redact(apiKey))
		fmt.Fprintf(stdout, "  model:    %s\n", model)
		fmt.Fprint(stdout, "Overwrite? [y/N]: ")
		yn, err := readLine(br)
		if err != nil {
			return err
		}
		if !isYes(yn) {
			fmt.Fprintln(stdout, "Aborted.")
			return nil
		}
	}

	newProfile := config.Profile{
		BaseURL: baseURL,
		APIKey:  apiKey,
		Model:   model,
	}

	// 6. Preview + confirm.
	preview, err := toml.Marshal(map[string]config.Profile{name: newProfile})
	if err != nil {
		return fmt.Errorf("preview: %w", err)
	}
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "The following will be written to", creds.Path+":")
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, string(redactAPIKeyTOML(preview)))
	fmt.Fprint(stdout, "Proceed? [Y/n]: ")
	yn, err := readLine(br)
	if err != nil {
		return err
	}
	if !isYesDefault(yn) {
		fmt.Fprintln(stdout, "Aborted.")
		return nil
	}

	// 7. Offer to promote to default if user named it something else and
	//    no default exists yet.
	if name != "default" {
		if _, has := creds.Profiles["default"]; !has {
			fmt.Fprint(stdout, "Make this your default profile? (renames to 'default') [y/N]: ")
			yn, err := readLine(br)
			if err != nil {
				return err
			}
			if isYes(yn) {
				name = "default"
			}
		}
	}

	creds.Profiles[name] = newProfile
	if err := creds.Save(); err != nil {
		return runtimeErrorf("save: %s", err)
	}
	fmt.Fprintf(stdout, "Wrote profile %q to %s\n", name, creds.Path)
	return nil
}

func readLine(br *bufio.Reader) (string, error) {
	line, err := br.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// readSecret reads a line with echo disabled when stdin is a TTY, falling
// back to plain readLine when it isn't (scripted input).
func readSecret(stdin io.Reader, stdout io.Writer, prompt string) (string, error) {
	f, ok := stdin.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		fmt.Fprint(stdout, prompt)
		return readLine(bufio.NewReader(stdin))
	}
	fmt.Fprint(stdout, prompt)
	b, err := term.ReadPassword(int(f.Fd()))
	fmt.Fprintln(stdout)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func redact(key string) string {
	if len(key) <= 6 {
		return "****"
	}
	return key[:4] + "…" + key[len(key)-2:]
}

// redactAPIKeyTOML rewrites any line starting with `api_key = "..."` to
// show a redacted form, so the preview doesn't spill the key back to the
// terminal (and any attached screen recorder).
func redactAPIKeyTOML(b []byte) []byte {
	out := make([]byte, 0, len(b))
	for _, line := range strings.Split(string(b), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "api_key") {
			// find the quoted value and redact it
			if i := strings.Index(line, `"`); i >= 0 {
				if j := strings.LastIndex(line, `"`); j > i {
					redacted := redact(line[i+1 : j])
					line = line[:i] + `"` + redacted + `"`
				}
			}
		}
		out = append(out, []byte(line)...)
		out = append(out, '\n')
	}
	return out
}

func isYes(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "y" || s == "yes"
}

// isYesDefault is for prompts whose default is yes (empty reply means yes).
func isYesDefault(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "" || s == "y" || s == "yes"
}
