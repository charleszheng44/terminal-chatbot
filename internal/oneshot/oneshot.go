package oneshot

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/zc/tchat/internal/config"
	"github.com/zc/tchat/internal/provider"
)

// Run executes a one-shot query: sends the prompt to the configured model,
// streams the response to stdout, and exits.
func Run(cfg *config.Config, prompt string) error {
	providerName, modelName, err := cfg.ResolveModel(cfg.DefaultModel)
	if err != nil {
		return fmt.Errorf("resolving model: %w", err)
	}

	providerCfg, ok := cfg.Providers[providerName]
	if !ok {
		return fmt.Errorf("provider %q not found in config", providerName)
	}

	p, err := provider.NewProvider(providerName, providerCfg)
	if err != nil {
		return fmt.Errorf("creating provider: %w", err)
	}

	opts := provider.ChatOptions{
		Model:       modelName,
		Temperature: cfg.Defaults.Temperature,
		MaxTokens:   cfg.Defaults.MaxTokens,
	}

	var messages []provider.Message
	if cfg.Defaults.SystemPrompt != "" {
		messages = append(messages, provider.Message{
			Role:    "system",
			Content: cfg.Defaults.SystemPrompt,
		})
	}
	messages = append(messages, provider.Message{
		Role:    "user",
		Content: prompt,
	})

	isTTY := isTerminal()
	ctx := context.Background()

	// applyDefaults guarantees Streaming is non-nil, but guard anyway to
	// keep Run safe for callers who build a Config directly.
	streaming := cfg.Defaults.Streaming == nil || *cfg.Defaults.Streaming
	if streaming {
		var buf strings.Builder
		err := p.StreamChat(ctx, messages, opts, func(token string) {
			buf.WriteString(token)
			fmt.Print(token)
		})
		if err != nil {
			return fmt.Errorf("streaming chat: %w", err)
		}
		fmt.Println()
		_ = buf.String() // collected but unused in streaming mode
		return nil
	}

	// Non-streaming mode.
	response, err := p.Chat(ctx, messages, opts)
	if err != nil {
		return fmt.Errorf("chat: %w", err)
	}

	if isTTY {
		rendered, renderErr := renderMarkdown(response)
		if renderErr == nil {
			fmt.Print(rendered)
			return nil
		}
		// Fall back to plain text on render failure.
	}

	fmt.Println(response)
	return nil
}

// isTerminal reports whether stdout is connected to a terminal.
func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// renderMarkdown renders text as markdown for terminal display using Glamour.
func renderMarkdown(text string) (string, error) {
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		return "", err
	}
	return renderer.Render(text)
}
