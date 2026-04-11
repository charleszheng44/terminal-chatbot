package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// newOpenAIStreamServer returns a httptest.Server that mimics OpenAI's SSE
// chat completion endpoint and streams the given tokens as separate delta
// chunks, followed by the [DONE] sentinel.
func newOpenAIStreamServer(t *testing.T, tokens []string) *httptest.Server {
	t.Helper()
	h := http.NewServeMux()
	h.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// Drain the request body.
		_, _ = io.Copy(io.Discard, r.Body)

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Errorf("ResponseWriter does not implement http.Flusher")
			return
		}
		w.WriteHeader(http.StatusOK)

		for _, tok := range tokens {
			chunk := fmt.Sprintf(
				`{"id":"c1","object":"chat.completion.chunk","created":1,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":%q},"finish_reason":null}]}`,
				tok,
			)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", chunk)
			flusher.Flush()
		}
		// Final done marker.
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func TestOpenAI_StreamChat_E2E(t *testing.T) {
	tokens := []string{"hello", " ", "world"}
	srv := newOpenAIStreamServer(t, tokens)

	p := NewOpenAI(ProviderConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL + "/v1",
	})

	var mu sync.Mutex
	var got strings.Builder
	err := p.StreamChat(
		context.Background(),
		[]Message{{Role: "user", Content: "hi"}},
		ChatOptions{Model: "gpt-4o", Temperature: 0.7, MaxTokens: 100},
		func(tok string) {
			mu.Lock()
			got.WriteString(tok)
			mu.Unlock()
		},
	)
	if err != nil {
		t.Fatalf("StreamChat err: %v", err)
	}
	if got.String() != "hello world" {
		t.Errorf("streamed content = %q, want 'hello world'", got.String())
	}
}

func TestOpenAI_Chat_E2E(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		// Minimal ChatCompletion response body expected by the go-openai SDK.
		_, _ = fmt.Fprint(w, `{
			"id":"c2",
			"object":"chat.completion",
			"created":1,
			"model":"gpt-4o",
			"choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
		}`)
	}))
	t.Cleanup(srv.Close)

	p := NewOpenAI(ProviderConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL + "/v1",
	})

	out, err := p.Chat(
		context.Background(),
		[]Message{{Role: "user", Content: "ping"}},
		ChatOptions{Model: "gpt-4o"},
	)
	if err != nil {
		t.Fatalf("Chat err: %v", err)
	}
	if out != "pong" {
		t.Errorf("Chat returned %q, want 'pong'", out)
	}
}

func TestOpenAI_ListModels_ReturnsConfigured(t *testing.T) {
	p := NewOpenAI(ProviderConfig{Models: []string{"m1", "m2"}})
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels err: %v", err)
	}
	if len(models) != 2 || models[0] != "m1" || models[1] != "m2" {
		t.Errorf("ListModels = %v", models)
	}
}
