package provider

import (
	"context"
	"errors"
	"io"

	openai "github.com/sashabaranov/go-openai"
)

type openaiProvider struct {
	cfg    ProviderConfig
	client *openai.Client
}

// NewOpenAI creates an OpenAI provider. If cfg.BaseURL is set, it is used as
// the API base URL, which allows OpenAI-compatible services (Ollama, DeepSeek,
// Kimi, etc.) to work transparently.
func NewOpenAI(cfg ProviderConfig) Provider {
	config := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		config.BaseURL = cfg.BaseURL
	}
	return &openaiProvider{
		cfg:    cfg,
		client: openai.NewClientWithConfig(config),
	}
}

// toOpenAIMessages converts internal Message types to the OpenAI SDK format.
func toOpenAIMessages(messages []Message) []openai.ChatCompletionMessage {
	out := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		out[i] = openai.ChatCompletionMessage{
			Role:    m.Role, // "user", "assistant", "system" map directly
			Content: m.Content,
		}
	}
	return out
}

// buildRequest creates a ChatCompletionRequest from messages and options.
func buildRequest(messages []Message, opts ChatOptions) openai.ChatCompletionRequest {
	req := openai.ChatCompletionRequest{
		Model:    opts.Model,
		Messages: toOpenAIMessages(messages),
	}
	if opts.Temperature > 0 {
		req.Temperature = float32(opts.Temperature)
	}
	if opts.MaxTokens > 0 {
		req.MaxTokens = opts.MaxTokens
	}
	return req
}

func (p *openaiProvider) StreamChat(ctx context.Context, messages []Message, opts ChatOptions, onToken func(string)) error {
	req := buildRequest(messages, opts)
	req.Stream = true

	stream, err := p.client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return err
	}
	defer func() { _ = stream.Close() }()

	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if len(resp.Choices) > 0 {
			delta := resp.Choices[0].Delta.Content
			if delta != "" {
				onToken(delta)
			}
		}
	}
}

func (p *openaiProvider) Chat(ctx context.Context, messages []Message, opts ChatOptions) (string, error) {
	req := buildRequest(messages, opts)

	resp, err := p.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", nil
	}
	return resp.Choices[0].Message.Content, nil
}

func (p *openaiProvider) ListModels(ctx context.Context) ([]string, error) {
	return p.cfg.Models, nil
}
