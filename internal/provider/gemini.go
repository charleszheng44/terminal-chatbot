package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type geminiProvider struct {
	cfg ProviderConfig
}

func NewGemini(cfg ProviderConfig) Provider {
	return &geminiProvider{cfg: cfg}
}

func (p *geminiProvider) newClient(ctx context.Context) (*genai.Client, error) {
	return genai.NewClient(ctx, option.WithAPIKey(p.cfg.APIKey))
}

// configureModel sets up the model with the given options and system instruction.
func (p *geminiProvider) configureModel(model *genai.GenerativeModel, messages []Message, opts ChatOptions) ([]Message, error) {
	if opts.Temperature != 0 {
		model.SetTemperature(float32(opts.Temperature))
	}
	if opts.MaxTokens != 0 {
		model.SetMaxOutputTokens(int32(opts.MaxTokens))
	}

	// Extract system messages and set as SystemInstruction.
	var systemParts []genai.Part
	var nonSystemMessages []Message
	for _, msg := range messages {
		if msg.Role == "system" {
			systemParts = append(systemParts, genai.Text(msg.Content))
		} else {
			nonSystemMessages = append(nonSystemMessages, msg)
		}
	}
	if len(systemParts) > 0 {
		model.SystemInstruction = &genai.Content{
			Parts: systemParts,
		}
	}

	return nonSystemMessages, nil
}

// buildHistory converts messages into Gemini Content history plus the final user parts.
// The last message must be from the user.
func buildHistory(messages []Message) ([]*genai.Content, []genai.Part, error) {
	if len(messages) == 0 {
		return nil, nil, fmt.Errorf("no messages provided")
	}

	var history []*genai.Content
	for _, msg := range messages[:len(messages)-1] {
		role := mapRole(msg.Role)
		history = append(history, &genai.Content{
			Role:  role,
			Parts: []genai.Part{genai.Text(msg.Content)},
		})
	}

	last := messages[len(messages)-1]
	return history, []genai.Part{genai.Text(last.Content)}, nil
}

func mapRole(role string) string {
	switch role {
	case "assistant":
		return "model"
	default:
		return "user"
	}
}

func (p *geminiProvider) StreamChat(ctx context.Context, messages []Message, opts ChatOptions, onToken func(string)) error {
	client, err := p.newClient(ctx)
	if err != nil {
		return fmt.Errorf("creating Gemini client: %w", err)
	}
	defer func() { _ = client.Close() }()

	model := client.GenerativeModel(opts.Model)
	nonSystemMessages, err := p.configureModel(model, messages, opts)
	if err != nil {
		return err
	}

	history, lastParts, err := buildHistory(nonSystemMessages)
	if err != nil {
		return err
	}

	cs := model.StartChat()
	cs.History = history

	iter := cs.SendMessageStream(ctx, lastParts...)
	for {
		resp, err := iter.Next()
		if err == iterator.Done {
			return nil
		}
		if err != nil {
			return fmt.Errorf("streaming Gemini response: %w", err)
		}
		for _, cand := range resp.Candidates {
			if cand.Content == nil {
				continue
			}
			for _, part := range cand.Content.Parts {
				if t, ok := part.(genai.Text); ok {
					onToken(string(t))
				}
			}
		}
	}
}

func (p *geminiProvider) Chat(ctx context.Context, messages []Message, opts ChatOptions) (string, error) {
	client, err := p.newClient(ctx)
	if err != nil {
		return "", fmt.Errorf("creating Gemini client: %w", err)
	}
	defer func() { _ = client.Close() }()

	model := client.GenerativeModel(opts.Model)
	nonSystemMessages, err := p.configureModel(model, messages, opts)
	if err != nil {
		return "", err
	}

	history, lastParts, err := buildHistory(nonSystemMessages)
	if err != nil {
		return "", err
	}

	cs := model.StartChat()
	cs.History = history

	resp, err := cs.SendMessage(ctx, lastParts...)
	if err != nil {
		return "", fmt.Errorf("gemini chat: %w", err)
	}

	var sb strings.Builder
	for _, cand := range resp.Candidates {
		if cand.Content == nil {
			continue
		}
		for _, part := range cand.Content.Parts {
			if t, ok := part.(genai.Text); ok {
				sb.WriteString(string(t))
			}
		}
	}
	return sb.String(), nil
}

func (p *geminiProvider) ListModels(ctx context.Context) ([]string, error) {
	return p.cfg.Models, nil
}
