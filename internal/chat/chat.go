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
	// lastUserInput is the most recent user input passed to Send, preserved
	// even when Send's rollback removes the corresponding history entry on
	// error so that Retry can resubmit it.
	lastUserInput string
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
	s.lastUserInput = input
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
	s.lastUserInput = input
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

// Retry resends the most recent user input. It handles both the normal case
// (a successful user+assistant exchange at the tail of history) and the
// post-failure case (the previous Send failed and rolled its user entry off
// the history tail). One code path handles both:
//
//  1. If history ends with an assistant reply preceded by a user message that
//     matches lastUserInput, that pair is the result of the most recent
//     successful Send: drop both so Send re-adds the user turn cleanly.
//  2. If history ends with a user message matching lastUserInput (e.g. a
//     partial history that was seeded externally), drop it.
//  3. Otherwise the last Send failed and already rolled back its own user
//     entry; leave history alone.
//
// In all three cases the retry is driven by lastUserInput, which is set on
// every Send call and preserved across Send's rollback. If no user message
// has ever been sent, Retry returns an error.
func (s *Session) Retry(ctx context.Context, onToken func(string)) (string, error) {
	s.mu.Lock()

	if s.lastUserInput == "" {
		// Nothing was ever sent; fall back to the legacy check so a
		// caller-seeded history with a trailing user message is still
		// retryable.
		if n := len(s.messages); n > 0 && s.messages[n-1].Role == "user" {
			userMsg := s.messages[n-1].Content
			s.messages = s.messages[:n-1]
			s.mu.Unlock()
			return s.Send(ctx, userMsg, onToken)
		}
		s.mu.Unlock()
		return "", errors.New("no user message to retry")
	}

	userMsg := s.lastUserInput

	// Normal successful-turn case: drop the assistant reply and the matching
	// user message it followed.
	if n := len(s.messages); n >= 2 &&
		s.messages[n-1].Role == "assistant" &&
		s.messages[n-2].Role == "user" &&
		s.messages[n-2].Content == userMsg {
		s.messages = s.messages[:n-2]
		s.mu.Unlock()
		return s.Send(ctx, userMsg, onToken)
	}

	// Partial/seeded history case: tail user message matches lastUserInput.
	if n := len(s.messages); n >= 1 &&
		s.messages[n-1].Role == "user" &&
		s.messages[n-1].Content == userMsg {
		s.messages = s.messages[:n-1]
		s.mu.Unlock()
		return s.Send(ctx, userMsg, onToken)
	}

	// Post-failure case: the last Send failed and already rolled back its
	// user entry. Just resend — Send will append lastUserInput fresh.
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
