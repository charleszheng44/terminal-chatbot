package provider

import (
	"context"
)

type anthropicProvider struct {
	cfg ProviderConfig
}

func NewAnthropic(cfg ProviderConfig) Provider {
	return &anthropicProvider{cfg: cfg}
}

func (p *anthropicProvider) StreamChat(ctx context.Context, messages []Message, opts ChatOptions, onToken func(string)) error {
	return nil
}

func (p *anthropicProvider) Chat(ctx context.Context, messages []Message, opts ChatOptions) (string, error) {
	return "", nil
}

func (p *anthropicProvider) ListModels(ctx context.Context) ([]string, error) {
	return p.cfg.Models, nil
}
