package provider

import (
	"context"
	"fmt"
)

// Message represents a chat message.
type Message struct {
	Role    string // "user", "assistant", "system"
	Content string
}

// ChatOptions configures a chat request.
type ChatOptions struct {
	Model       string
	Temperature float64
	MaxTokens   int
}

// Provider is the interface that all LLM providers implement.
type Provider interface {
	// StreamChat sends a conversation and streams tokens back via onToken callback.
	StreamChat(ctx context.Context, messages []Message, opts ChatOptions, onToken func(string)) error
	// Chat sends a conversation and returns the full response.
	Chat(ctx context.Context, messages []Message, opts ChatOptions) (string, error)
	// ListModels returns the available models for this provider.
	ListModels(ctx context.Context) ([]string, error)
}

// ProviderConfig holds the configuration for a single provider.
type ProviderConfig struct {
	APIKey  string   `yaml:"api_key"`
	BaseURL string   `yaml:"base_url"`
	Models  []string `yaml:"models"`
}

// NewProvider creates a provider by name using the given config.
func NewProvider(name string, cfg ProviderConfig) (Provider, error) {
	switch name {
	case "openai":
		return NewOpenAI(cfg), nil
	case "anthropic":
		return NewAnthropic(cfg), nil
	case "gemini":
		return NewGemini(cfg), nil
	default:
		// Providers with base_url are treated as OpenAI-compatible
		if cfg.BaseURL != "" {
			return NewOpenAI(cfg), nil
		}
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
}
