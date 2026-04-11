package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newAnthropicStreamServer streams a sequence of text_delta tokens using the
// Anthropic Messages SSE event format.
func newAnthropicStreamServer(t *testing.T, tokens []string) *httptest.Server {
	t.Helper()
	h := http.NewServeMux()
	h.HandleFunc("/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		_, _ = io.Copy(io.Discard, r.Body)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Errorf("ResponseWriter does not implement http.Flusher")
			return
		}
		w.WriteHeader(http.StatusOK)

		writeEvent := func(event, data string) {
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
			flusher.Flush()
		}

		// message_start
		writeEvent("message_start", `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-test","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":0}}}`)
		// content_block_start
		writeEvent("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
		// content_block_delta for each token
		for _, tok := range tokens {
			data := fmt.Sprintf(
				`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":%q}}`,
				tok,
			)
			writeEvent("content_block_delta", data)
		}
		// content_block_stop
		writeEvent("content_block_stop", `{"type":"content_block_stop","index":0}`)
		// message_delta with stop reason
		writeEvent("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":3}}`)
		// message_stop
		writeEvent("message_stop", `{"type":"message_stop"}`)
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func TestAnthropic_StreamChat_E2E(t *testing.T) {
	tokens := []string{"hello", " ", "world"}
	srv := newAnthropicStreamServer(t, tokens)

	p := NewAnthropic(ProviderConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})

	var got strings.Builder
	err := p.StreamChat(
		context.Background(),
		[]Message{{Role: "user", Content: "hi"}},
		ChatOptions{Model: "claude-test", MaxTokens: 100},
		func(tok string) {
			got.WriteString(tok)
		},
	)
	if err != nil {
		t.Fatalf("StreamChat err: %v", err)
	}
	if got.String() != "hello world" {
		t.Errorf("streamed content = %q, want 'hello world'", got.String())
	}
}

func TestAnthropic_Chat_E2E(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"id":"msg_2",
			"type":"message",
			"role":"assistant",
			"model":"claude-test",
			"content":[{"type":"text","text":"pong"}],
			"stop_reason":"end_turn",
			"stop_sequence":null,
			"usage":{"input_tokens":1,"output_tokens":1}
		}`)
	}))
	t.Cleanup(srv.Close)

	p := NewAnthropic(ProviderConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})

	out, err := p.Chat(
		context.Background(),
		[]Message{{Role: "user", Content: "ping"}},
		ChatOptions{Model: "claude-test", MaxTokens: 100},
	)
	if err != nil {
		t.Fatalf("Chat err: %v", err)
	}
	if out != "pong" {
		t.Errorf("Chat returned %q, want 'pong'", out)
	}
}

func TestAnthropic_ListModels_ReturnsConfigured(t *testing.T) {
	p := NewAnthropic(ProviderConfig{Models: []string{"claude-a", "claude-b"}})
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels err: %v", err)
	}
	if len(models) != 2 {
		t.Errorf("ListModels = %v", models)
	}
}
