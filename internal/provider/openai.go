package provider

import (
	"context"
)

type openaiProvider struct {
	cfg ProviderConfig
}

func NewOpenAI(cfg ProviderConfig) Provider {
	return &openaiProvider{cfg: cfg}
}

func (p *openaiProvider) StreamChat(ctx context.Context, messages []Message, opts ChatOptions, onToken func(string)) error {
	return nil
}

func (p *openaiProvider) Chat(ctx context.Context, messages []Message, opts ChatOptions) (string, error) {
	return "", nil
}

func (p *openaiProvider) ListModels(ctx context.Context) ([]string, error) {
	return p.cfg.Models, nil
}
