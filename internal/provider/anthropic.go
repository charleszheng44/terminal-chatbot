package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type anthropicProvider struct {
	cfg    ProviderConfig
	client anthropic.Client
}

func NewAnthropic(cfg ProviderConfig) Provider {
	opts := []option.RequestOption{
		option.WithAPIKey(cfg.APIKey),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	client := anthropic.NewClient(opts...)
	return &anthropicProvider{cfg: cfg, client: client}
}

// buildParams converts our generic types into Anthropic SDK parameters.
// System messages are extracted and passed separately via the System field.
func (p *anthropicProvider) buildParams(messages []Message, opts ChatOptions) anthropic.MessageNewParams {
	var system []anthropic.TextBlockParam
	var msgs []anthropic.MessageParam

	for _, m := range messages {
		switch m.Role {
		case "system":
			system = append(system, anthropic.TextBlockParam{Text: m.Content})
		case "user":
			msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		case "assistant":
			msgs = append(msgs, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
		}
	}

	maxTokens := int64(opts.MaxTokens)
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(opts.Model),
		MaxTokens: maxTokens,
		Messages:  msgs,
	}

	if len(system) > 0 {
		params.System = system
	}

	if opts.Temperature > 0 {
		params.Temperature = anthropic.Float(opts.Temperature)
	}

	return params
}

func (p *anthropicProvider) StreamChat(ctx context.Context, messages []Message, opts ChatOptions, onToken func(string)) error {
	params := p.buildParams(messages, opts)

	stream := p.client.Messages.NewStreaming(ctx, params)
	defer func() { _ = stream.Close() }()

	for stream.Next() {
		event := stream.Current()
		// We only care about content_block_delta events with text deltas.
		if event.Type == "content_block_delta" && event.Delta.Type == "text_delta" {
			onToken(event.Delta.Text)
		}
	}

	if err := stream.Err(); err != nil {
		return fmt.Errorf("anthropic streaming error: %w", err)
	}

	return nil
}

func (p *anthropicProvider) Chat(ctx context.Context, messages []Message, opts ChatOptions) (string, error) {
	params := p.buildParams(messages, opts)

	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("anthropic chat error: %w", err)
	}

	var sb strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}

	return sb.String(), nil
}

func (p *anthropicProvider) ListModels(ctx context.Context) ([]string, error) {
	return p.cfg.Models, nil
}
