package chat

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/zc/tchat/internal/provider"
)

// Session manages conversation state for an interactive chat.
type Session struct {
	mu       sync.Mutex
	provider provider.Provider
	opts     provider.ChatOptions
	messages []provider.Message
	stream   bool
}

// NewSession creates a new chat session with the given provider and options.
func NewSession(p provider.Provider, opts provider.ChatOptions) *Session {
	return &Session{
		provider: p,
		opts:     opts,
		messages: nil,
		stream:   true,
	}
}

// Send adds a user message, streams the response via onToken, appends the
// assistant reply to history, and returns the full response.
func (s *Session) Send(ctx context.Context, input string, onToken func(string)) (string, error) {
	s.mu.Lock()
	s.messages = append(s.messages, provider.Message{Role: "user", Content: input})
	msgs := make([]provider.Message, len(s.messages))
	copy(msgs, s.messages)
	streaming := s.stream
	opts := s.opts
	p := s.provider
	s.mu.Unlock()

	if !streaming {
		return s.doChat(ctx, p, msgs, opts)
	}

	var buf strings.Builder
	err := p.StreamChat(ctx, msgs, opts, func(token string) {
		buf.WriteString(token)
		if onToken != nil {
			onToken(token)
		}
	})
	if err != nil {
		// Remove the user message we just added since the call failed.
		s.mu.Lock()
		if len(s.messages) > 0 && s.messages[len(s.messages)-1].Role == "user" {
			s.messages = s.messages[:len(s.messages)-1]
		}
		s.mu.Unlock()
		return "", err
	}

	response := buf.String()
	s.mu.Lock()
	s.messages = append(s.messages, provider.Message{Role: "assistant", Content: response})
	s.mu.Unlock()
	return response, nil
}

// SendNoStream sends a message without streaming.
func (s *Session) SendNoStream(ctx context.Context, input string) (string, error) {
	s.mu.Lock()
	s.messages = append(s.messages, provider.Message{Role: "user", Content: input})
	msgs := make([]provider.Message, len(s.messages))
	copy(msgs, s.messages)
	opts := s.opts
	p := s.provider
	s.mu.Unlock()

	return s.doChat(ctx, p, msgs, opts)
}

func (s *Session) doChat(ctx context.Context, p provider.Provider, msgs []provider.Message, opts provider.ChatOptions) (string, error) {
	response, err := p.Chat(ctx, msgs, opts)
	if err != nil {
		s.mu.Lock()
		if len(s.messages) > 0 && s.messages[len(s.messages)-1].Role == "user" {
			s.messages = s.messages[:len(s.messages)-1]
		}
		s.mu.Unlock()
		return "", err
	}

	s.mu.Lock()
	s.messages = append(s.messages, provider.Message{Role: "assistant", Content: response})
	s.mu.Unlock()
	return response, nil
}

// SetProvider switches the underlying provider.
func (s *Session) SetProvider(p provider.Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.provider = p
}

// SetModel updates the model name sent on future requests. This must be
// called alongside SetProvider when switching between providers, otherwise
// the old model name will be sent to the new provider.
func (s *Session) SetModel(model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opts.Model = model
}

// SetSystemPrompt sets or updates the system prompt. It is stored as the first
// message with role "system".
func (s *Session) SetSystemPrompt(prompt string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.messages) > 0 && s.messages[0].Role == "system" {
		if prompt == "" {
			s.messages = s.messages[1:]
		} else {
			s.messages[0].Content = prompt
		}
	} else if prompt != "" {
		s.messages = append([]provider.Message{{Role: "system", Content: prompt}}, s.messages...)
	}
}

// SystemPrompt returns the current system prompt, or empty string if none is set.
func (s *Session) SystemPrompt() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.messages) > 0 && s.messages[0].Role == "system" {
		return s.messages[0].Content
	}
	return ""
}

// SetTemperature updates the temperature for future requests.
func (s *Session) SetTemperature(t float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opts.Temperature = t
}

// Temperature returns the current temperature setting.
func (s *Session) Temperature() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.opts.Temperature
}

// SetStreaming enables or disables streaming mode.
func (s *Session) SetStreaming(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stream = enabled
}

// Streaming returns whether streaming is enabled.
func (s *Session) Streaming() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stream
}

// Clear removes all messages from history, keeping the system prompt if one
// is set.
func (s *Session) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	var kept []provider.Message
	if len(s.messages) > 0 && s.messages[0].Role == "system" {
		kept = append(kept, s.messages[0])
	}
	s.messages = kept
}

// History returns a copy of the message history.
func (s *Session) History() []provider.Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]provider.Message, len(s.messages))
	copy(out, s.messages)
	return out
}

// Retry resends the most recent user message. It handles the normal
// successful-turn case and the post-failure case through a single code path:
//
//   - If the last message is an assistant reply, the trailing assistant +
//     user pair is removed and that user content is Sent again.
//   - If the last message is a user message (e.g. a prior Send failed but
//     history still ends on a user turn, or a partial conversation was
//     loaded), that trailing user message is removed and its content is
//     Sent again.
//
// Either way, Send re-appends the user message and drives a fresh provider
// call. Returns an error if there is no user message anywhere in history.
func (s *Session) Retry(ctx context.Context, onToken func(string)) (string, error) {
	s.mu.Lock()

	// Find the most recent user message.
	userIdx := -1
	for i := len(s.messages) - 1; i >= 0; i-- {
		if s.messages[i].Role == "user" {
			userIdx = i
			break
		}
	}
	if userIdx == -1 {
		s.mu.Unlock()
		return "", errors.New("no user message to retry")
	}

	userMsg := s.messages[userIdx].Content
	// Trim from userIdx onward so Send can re-append the user turn cleanly
	// without duplicating it.
	s.messages = s.messages[:userIdx]
	s.mu.Unlock()

	return s.Send(ctx, userMsg, onToken)
}

// AddMessage appends a message to the history. This is useful for loading
// saved conversations without calling the provider.
func (s *Session) AddMessage(msg provider.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, msg)
}

// LastAssistantMessage returns the content of the most recent assistant message,
// or empty string if there are none.
func (s *Session) LastAssistantMessage() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := len(s.messages) - 1; i >= 0; i-- {
		if s.messages[i].Role == "assistant" {
			return s.messages[i].Content
		}
	}
	return ""
}
