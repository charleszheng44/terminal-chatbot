package provider

import (
	"context"
)

type geminiProvider struct {
	cfg ProviderConfig
}

func NewGemini(cfg ProviderConfig) Provider {
	return &geminiProvider{cfg: cfg}
}

func (p *geminiProvider) StreamChat(ctx context.Context, messages []Message, opts ChatOptions, onToken func(string)) error {
	return nil
}

func (p *geminiProvider) Chat(ctx context.Context, messages []Message, opts ChatOptions) (string, error) {
	return "", nil
}

func (p *geminiProvider) ListModels(ctx context.Context) ([]string, error) {
	return p.cfg.Models, nil
}
