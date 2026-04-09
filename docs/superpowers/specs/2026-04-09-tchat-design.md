# tchat — Terminal Chatbot Design Spec

## Overview

`tchat` is a terminal-based LLM chatbot that provides a unified interface to multiple LLM providers (OpenAI, Anthropic, Google Gemini) with rich markdown rendering, slash commands, and streaming responses. Built in Go using the Charm ecosystem.

## Goals

- Single binary, cross-platform CLI that works on all mainstream terminals
- Configurable multi-provider support with easy model switching
- Claude Code-style slash command interface
- Rich terminal UI with markdown rendering and syntax-highlighted code blocks
- Both interactive chat and one-shot query modes

## Tech Stack

- **Language:** Go
- **TUI framework:** [Bubble Tea](https://github.com/charmbracelet/bubbletea) (Elm-architecture TUI)
- **Styling:** [Lip Gloss](https://github.com/charmbracelet/lipgloss)
- **Markdown rendering:** [Glamour](https://github.com/charmbracelet/glamour)
- **CLI framework:** [Cobra](https://github.com/spf13/cobra)
- **Config parsing:** [Viper](https://github.com/spf13/viper) or direct YAML unmarshalling

## Architecture

```
tchat
├── cmd/                  # CLI entrypoint (cobra)
├── internal/
│   ├── config/           # Config loading, validation, defaults
│   ├── provider/         # LLM provider abstraction + implementations
│   │   ├── provider.go   # Interface definition
│   │   ├── openai.go     # OpenAI + OpenAI-compatible
│   │   ├── anthropic.go  # Claude
│   │   └── gemini.go     # Google Gemini
│   ├── chat/             # Conversation state, message history
│   ├── command/          # Slash command registry + handlers
│   └── ui/               # Bubble Tea TUI (input, output, layout)
├── config.example.yaml
├── go.mod
└── main.go
```

### Core Flow

1. User launches `tchat` (interactive) or `tchat "prompt"` (one-shot)
2. Config is loaded from `~/.config/tchat/config.yaml`
3. Provider is initialized based on active model in config
4. TUI starts (or one-shot handler runs), user input goes through command parser
5. If input starts with `/`, it's routed to the command handler
6. Otherwise, it's sent to the active provider, response streams back with markdown rendering

## Provider Abstraction

```go
type Provider interface {
    // Send a conversation and stream tokens back via callback
    StreamChat(ctx context.Context, messages []Message, opts ChatOptions, onToken func(string)) error
    // Send a conversation and return full response
    Chat(ctx context.Context, messages []Message, opts ChatOptions) (string, error)
    // List available models for this provider
    ListModels(ctx context.Context) ([]string, error)
}

type Message struct {
    Role    string // "user", "assistant", "system"
    Content string
}

type ChatOptions struct {
    Model       string
    Temperature float64
    MaxTokens   int
}
```

### Provider Implementations

- **OpenAI** — uses `github.com/sashabaranov/go-openai`. Supports custom `base_url` in config, so it doubles as a generic OpenAI-compatible provider for Ollama, Kimi, DeepSeek, Qwen, etc.
- **Anthropic** — uses `github.com/anthropics/anthropic-sdk-go` (official Go SDK).
- **Gemini** — uses `github.com/google/generative-ai-go`.

Each provider is constructed from its config section and implements the same interface. The active provider is selected based on which model is currently set.

## Streaming

Streaming works in three layers:

### Layer 1: Provider

Each provider uses its SDK's native streaming API and calls `onToken(chunk)` for each received token. All three SDKs support streaming natively — OpenAI uses SSE, Anthropic uses SSE with different event types, Gemini uses its own stream format. The provider layer normalizes all of these into the same `onToken` callback.

### Layer 2: Chat Manager

Orchestrates the call, collects the full response for conversation history, and forwards tokens to the UI:

```
User input -> Chat.Send() -> provider.StreamChat(onToken) -> UI update per token
                           -> appends full response to history when done
```

### Layer 3: Bubble Tea UI

The TUI receives tokens via `tea.Msg`. Each token triggers a re-render of the response area.

### Debouncing

Markdown re-rendering on every single token would be expensive. Tokens are buffered and the markdown is re-rendered every ~50ms, with a final render when streaming completes. Raw text is appended immediately for responsiveness, and the styled markdown replaces it on each render tick.

### Non-streaming Fallback

When streaming is disabled in config, `Chat()` is called instead. A spinner is shown, then the full response is rendered at once.

## Configuration

Single YAML file at `~/.config/tchat/config.yaml`:

```yaml
# Active model on startup
default_model: openai/gpt-4o

# Global defaults
defaults:
  temperature: 0.7
  max_tokens: 4096
  streaming: true
  system_prompt: ""

# Provider configurations
providers:
  openai:
    api_key: "sk-..."
    models:
      - gpt-4o
      - gpt-4o-mini
      - o1

  anthropic:
    api_key: "sk-ant-..."
    models:
      - claude-sonnet-4-20250514
      - claude-haiku-4-20250414

  gemini:
    api_key: "AI..."
    models:
      - gemini-2.0-flash
      - gemini-2.5-pro

  # OpenAI-compatible services via custom base_url
  deepseek:
    api_key: "sk-..."
    base_url: "https://api.deepseek.com/v1"
    models:
      - deepseek-chat
      - deepseek-reasoner

# Model aliases for quick switching
aliases:
  gpt: openai/gpt-4o
  claude: anthropic/claude-sonnet-4-20250514
  fast: openai/gpt-4o-mini
```

### Model Naming

`provider/model-name` format. `/model claude` resolves via alias, `/model anthropic/claude-sonnet-4-20250514` uses the full name. Any provider with a `base_url` field is treated as OpenAI-compatible.

## Slash Commands

| Command | Description |
|---------|-------------|
| `/help` | Show all available commands |
| `/model [name]` | Switch model (no arg = list available models) |
| `/config` | Open config file in `$EDITOR` |
| `/system [prompt]` | Set/show system prompt for current session |
| `/history` | Show conversation history |
| `/clear` | Clear conversation history |
| `/copy` | Copy last assistant response to clipboard |
| `/save [path]` | Export conversation to file (default: `~/tchat-export-<timestamp>.md`) |
| `/load [path]` | Load a saved conversation |
| `/retry` | Regenerate last response |
| `/temp [value]` | Get/set temperature for current session |
| `/exit` | Quit (also Ctrl+C, Ctrl+D) |

### Command Registry

Commands are registered as a map of name to handler function:

```go
type Command struct {
    Name        string
    Description string
    Handler     func(app *App, args string) tea.Cmd
}
```

Input parsing: if a line starts with `/`, split on first space to get command name + args. Otherwise, treat as a chat message.

## TUI Layout

```
┌──────────────────────────────────────────┐
│ tchat — openai/gpt-4o           streaming│  <- status bar
├──────────────────────────────────────────┤
│                                          │
│ You: What is a goroutine?                │
│                                          │
│ Assistant:                               │
│ A **goroutine** is a lightweight thread  │
│ managed by the Go runtime...             │
│                                          │
│ ```go                                    │
│ go func() {                              │
│     fmt.Println("hello")                 │
│ }()                                      │
│ ```                                      │
│                                          │  <- scrollable chat area
├──────────────────────────────────────────┤
│ > Type a message... (/help for commands) │  <- input area (multi-line)
└──────────────────────────────────────────┘
```

- **Status bar** — shows current model, streaming indicator (spinner while generating)
- **Chat area** — scrollable viewport, markdown rendered via Glamour, syntax-highlighted code blocks
- **Input area** — multi-line text input. Enter sends, Shift+Enter for newline. Resizes up to ~5 lines.

## Keybindings

| Key | Action |
|-----|--------|
| Enter | Send message |
| Shift+Enter | New line |
| Ctrl+X, Ctrl+E | Open prompt in `$EDITOR` (chord, same as zsh) |
| Ctrl+C | Cancel streaming / exit |
| Ctrl+D | Exit |
| Up / Down | Cycle through input history (readline-style) |

The Ctrl+X, Ctrl+E chord is handled by a two-step key state in the Bubble Tea key handler — Ctrl+X sets a flag, then if Ctrl+E follows within a short window it triggers the external editor. The editor is launched by suspending the Bubble Tea program, opening `$EDITOR` (or `$VISUAL`, falling back to `vim`) with the current input in a temp file, then resuming the TUI when the editor exits.

## CLI Modes

### Interactive Mode (default)

```bash
tchat
```

Launches the full TUI with chat area, input, status bar.

### One-Shot Mode

```bash
tchat "What is a goroutine?"
```

Sends the prompt to the active model, streams the response to stdout (with markdown rendering if stdout is a TTY, plain text if piped), then exits. No TUI — just stdin/stdout.
