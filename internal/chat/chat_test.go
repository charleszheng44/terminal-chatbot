package chat

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/zc/tchat/internal/provider"
)

// testProvider is a fake provider.Provider used to drive Session tests
// without touching the network.
type testProvider struct {
	tokens     []string // tokens streamed by StreamChat
	chatReply  string   // reply returned by Chat
	streamErr  error    // error returned from StreamChat
	chatErr    error    // error returned from Chat
	lastMsgs   []provider.Message
	lastOpts   provider.ChatOptions
	streamCall int
	chatCall   int
}

func (p *testProvider) StreamChat(ctx context.Context, messages []provider.Message, opts provider.ChatOptions, onToken func(string)) error {
	p.streamCall++
	p.lastMsgs = append([]provider.Message(nil), messages...)
	p.lastOpts = opts
	if p.streamErr != nil {
		return p.streamErr
	}
	for _, tok := range p.tokens {
		onToken(tok)
	}
	return nil
}

func (p *testProvider) Chat(ctx context.Context, messages []provider.Message, opts provider.ChatOptions) (string, error) {
	p.chatCall++
	p.lastMsgs = append([]provider.Message(nil), messages...)
	p.lastOpts = opts
	if p.chatErr != nil {
		return "", p.chatErr
	}
	return p.chatReply, nil
}

func (p *testProvider) ListModels(ctx context.Context) ([]string, error) {
	return nil, nil
}

func newTestSession(p provider.Provider) *Session {
	return NewSession(p, provider.ChatOptions{
		Model:       "test-model",
		Temperature: 0.5,
		MaxTokens:   100,
	})
}

func TestSession_Send_Streaming_AppendsHistory(t *testing.T) {
	p := &testProvider{tokens: []string{"hello", " ", "world"}}
	sess := newTestSession(p)

	var captured strings.Builder
	out, err := sess.Send(context.Background(), "hi", func(tok string) {
		captured.WriteString(tok)
	})
	if err != nil {
		t.Fatalf("Send err: %v", err)
	}
	if out != "hello world" {
		t.Errorf("Send returned %q, want 'hello world'", out)
	}
	if captured.String() != "hello world" {
		t.Errorf("onToken captured %q", captured.String())
	}

	hist := sess.History()
	if len(hist) != 2 {
		t.Fatalf("history len = %d, want 2", len(hist))
	}
	if hist[0].Role != "user" || hist[0].Content != "hi" {
		t.Errorf("hist[0] = %+v", hist[0])
	}
	if hist[1].Role != "assistant" || hist[1].Content != "hello world" {
		t.Errorf("hist[1] = %+v", hist[1])
	}
}

func TestSession_Send_StreamingError_RemovesUserMessage(t *testing.T) {
	p := &testProvider{streamErr: errors.New("boom")}
	sess := newTestSession(p)

	// Seed a prior turn so we can verify the rollback only removes the
	// newly-added user message, not earlier history.
	sess.AddMessage(provider.Message{Role: "user", Content: "old"})
	sess.AddMessage(provider.Message{Role: "assistant", Content: "reply"})
	before := len(sess.History())

	_, err := sess.Send(context.Background(), "will fail", nil)
	if err == nil {
		t.Fatal("expected error from Send")
	}

	after := len(sess.History())
	if after != before {
		t.Errorf("history len changed: before=%d after=%d", before, after)
	}
	hist := sess.History()
	if hist[len(hist)-1].Content == "will fail" {
		t.Errorf("failed user message was not rolled back")
	}
}

func TestSession_SendNoStream_Success(t *testing.T) {
	p := &testProvider{chatReply: "non-streamed reply"}
	sess := newTestSession(p)
	sess.SetStreaming(false)

	out, err := sess.Send(context.Background(), "q", nil)
	if err != nil {
		t.Fatalf("Send err: %v", err)
	}
	if out != "non-streamed reply" {
		t.Errorf("out = %q", out)
	}
	if p.chatCall != 1 || p.streamCall != 0 {
		t.Errorf("expected Chat called, got chat=%d stream=%d", p.chatCall, p.streamCall)
	}
	if len(sess.History()) != 2 {
		t.Errorf("history len = %d", len(sess.History()))
	}
}

func TestSession_SendNoStream_Error_RemovesUserMessage(t *testing.T) {
	p := &testProvider{chatErr: errors.New("boom")}
	sess := newTestSession(p)
	sess.SetStreaming(false)

	_, err := sess.Send(context.Background(), "q", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if len(sess.History()) != 0 {
		t.Errorf("history should be empty, got %d", len(sess.History()))
	}
}

func TestSession_SetProvider(t *testing.T) {
	p1 := &testProvider{tokens: []string{"a"}}
	p2 := &testProvider{tokens: []string{"b"}}
	sess := newTestSession(p1)

	if _, err := sess.Send(context.Background(), "q1", nil); err != nil {
		t.Fatal(err)
	}
	if p1.streamCall != 1 {
		t.Errorf("p1 stream calls = %d", p1.streamCall)
	}

	sess.SetProvider(p2)
	if _, err := sess.Send(context.Background(), "q2", nil); err != nil {
		t.Fatal(err)
	}
	if p2.streamCall != 1 {
		t.Errorf("p2 stream calls = %d", p2.streamCall)
	}
	if p1.streamCall != 1 {
		t.Errorf("p1 stream calls after SetProvider = %d, want 1", p1.streamCall)
	}
}

func TestSession_SetModel(t *testing.T) {
	p := &testProvider{tokens: []string{"x"}}
	sess := newTestSession(p)

	sess.SetModel("new-model")
	if _, err := sess.Send(context.Background(), "q", nil); err != nil {
		t.Fatal(err)
	}
	if p.lastOpts.Model != "new-model" {
		t.Errorf("model passed to provider = %q", p.lastOpts.Model)
	}
}

func TestSession_SystemPrompt_SetAndClear(t *testing.T) {
	p := &testProvider{tokens: []string{"ok"}}
	sess := newTestSession(p)

	if sess.SystemPrompt() != "" {
		t.Errorf("initial SystemPrompt non-empty")
	}
	sess.SetSystemPrompt("you are a cat")
	if sess.SystemPrompt() != "you are a cat" {
		t.Errorf("SystemPrompt after set = %q", sess.SystemPrompt())
	}
	hist := sess.History()
	if len(hist) != 1 || hist[0].Role != "system" {
		t.Fatalf("history after SetSystemPrompt = %+v", hist)
	}

	// Overwriting updates in place.
	sess.SetSystemPrompt("you are a dog")
	if sess.SystemPrompt() != "you are a dog" {
		t.Errorf("SystemPrompt after second set = %q", sess.SystemPrompt())
	}
	if len(sess.History()) != 1 {
		t.Errorf("history len after overwrite = %d", len(sess.History()))
	}

	// Clearing removes it.
	sess.SetSystemPrompt("")
	if sess.SystemPrompt() != "" {
		t.Errorf("SystemPrompt after clear = %q", sess.SystemPrompt())
	}
	if len(sess.History()) != 0 {
		t.Errorf("history len after clear = %d", len(sess.History()))
	}
}

func TestSession_Temperature_RoundTrip(t *testing.T) {
	p := &testProvider{tokens: []string{"ok"}}
	sess := newTestSession(p)
	if sess.Temperature() != 0.5 {
		t.Errorf("initial temp = %v", sess.Temperature())
	}
	sess.SetTemperature(0.9)
	if sess.Temperature() != 0.9 {
		t.Errorf("after set = %v", sess.Temperature())
	}
}

func TestSession_Streaming_Toggle(t *testing.T) {
	p := &testProvider{tokens: []string{"ok"}}
	sess := newTestSession(p)
	if !sess.Streaming() {
		t.Errorf("initial streaming should be true")
	}
	sess.SetStreaming(false)
	if sess.Streaming() {
		t.Errorf("after disable still true")
	}
}

func TestSession_Retry_HappyPath(t *testing.T) {
	p := &testProvider{tokens: []string{"first"}}
	sess := newTestSession(p)

	if _, err := sess.Send(context.Background(), "question", nil); err != nil {
		t.Fatal(err)
	}
	if len(sess.History()) != 2 {
		t.Fatalf("after Send len = %d", len(sess.History()))
	}

	// Swap tokens so we can tell the retry happened.
	p.tokens = []string{"second"}
	out, err := sess.Retry(context.Background(), nil)
	if err != nil {
		t.Fatalf("Retry err: %v", err)
	}
	if out != "second" {
		t.Errorf("Retry returned %q", out)
	}

	hist := sess.History()
	if len(hist) != 2 {
		t.Fatalf("after Retry len = %d", len(hist))
	}
	if hist[0].Content != "question" {
		t.Errorf("user message lost: %+v", hist[0])
	}
	if hist[1].Content != "second" {
		t.Errorf("assistant message = %q", hist[1].Content)
	}
}

func TestSession_Retry_EmptyHistoryErrors(t *testing.T) {
	sess := newTestSession(&testProvider{})
	if _, err := sess.Retry(context.Background(), nil); err == nil {
		t.Fatal("expected error on empty history")
	}
}

func TestSession_Retry_NoAssistantErrors(t *testing.T) {
	sess := newTestSession(&testProvider{})
	// Two user messages, no assistant — e.g. a failed Send rolling back
	// shouldn't happen with this sequence, but we still defend.
	sess.AddMessage(provider.Message{Role: "user", Content: "a"})
	sess.AddMessage(provider.Message{Role: "user", Content: "b"})
	if _, err := sess.Retry(context.Background(), nil); err == nil {
		t.Fatal("expected error when last message is not assistant")
	}
}

func TestSession_Clear_KeepsSystemPrompt(t *testing.T) {
	p := &testProvider{tokens: []string{"ok"}}
	sess := newTestSession(p)
	sess.SetSystemPrompt("system")
	if _, err := sess.Send(context.Background(), "hi", nil); err != nil {
		t.Fatal(err)
	}
	if len(sess.History()) != 3 {
		t.Fatalf("pre-clear len = %d", len(sess.History()))
	}

	sess.Clear()
	hist := sess.History()
	if len(hist) != 1 || hist[0].Role != "system" || hist[0].Content != "system" {
		t.Errorf("Clear lost system prompt: %+v", hist)
	}
}

func TestSession_Clear_NoSystemPrompt(t *testing.T) {
	p := &testProvider{tokens: []string{"ok"}}
	sess := newTestSession(p)
	if _, err := sess.Send(context.Background(), "hi", nil); err != nil {
		t.Fatal(err)
	}
	sess.Clear()
	if len(sess.History()) != 0 {
		t.Errorf("Clear should wipe non-system history")
	}
}

func TestSession_LastAssistantMessage(t *testing.T) {
	sess := newTestSession(&testProvider{})
	if got := sess.LastAssistantMessage(); got != "" {
		t.Errorf("initial LastAssistantMessage = %q", got)
	}
	sess.AddMessage(provider.Message{Role: "user", Content: "q"})
	sess.AddMessage(provider.Message{Role: "assistant", Content: "a1"})
	sess.AddMessage(provider.Message{Role: "user", Content: "q2"})
	sess.AddMessage(provider.Message{Role: "assistant", Content: "a2"})
	if got := sess.LastAssistantMessage(); got != "a2" {
		t.Errorf("LastAssistantMessage = %q", got)
	}
}
