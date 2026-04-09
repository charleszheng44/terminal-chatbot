# tchat

A terminal-based multi-LLM chatbot with streaming, markdown rendering, and slash commands. Supports OpenAI, Anthropic, Gemini, and OpenAI-compatible services.

## Features

- **Multi-provider** — OpenAI, Anthropic (Claude), Google Gemini, plus any OpenAI-compatible API (Ollama, DeepSeek, Kimi, etc.)
- **Streaming** — real-time token streaming with debounced markdown rendering
- **Rich TUI** — syntax-highlighted code blocks, styled markdown, scrollable chat
- **Slash commands** — `/model`, `/system`, `/save`, `/load`, `/retry`, and more
- **One-shot mode** — `tchat "prompt"` for quick queries
- **External editor** — Ctrl+X, Ctrl+E to compose prompts in your `$EDITOR`
- **Model aliases** — switch models with short names like `/model claude`

## Installation

```bash
go install github.com/zc/tchat@latest
```

Or build from source:

```bash
git clone https://github.com/charleszheng44/terminal-chatbot.git
cd terminal-chatbot
go build -o tchat .
```

## Configuration

Create `~/.config/tchat/config.yaml`:

```yaml
default_model: openai/gpt-4o

defaults:
  temperature: 0.7
  max_tokens: 4096
  streaming: true

providers:
  openai:
    api_key: "sk-..."
    models:
      - gpt-4o
      - gpt-4o-mini

  anthropic:
    api_key: "sk-ant-..."
    models:
      - claude-sonnet-4-20250514

  gemini:
    api_key: "AI..."
    models:
      - gemini-2.0-flash

  # Any OpenAI-compatible service
  deepseek:
    api_key: "sk-..."
    base_url: "https://api.deepseek.com/v1"
    models:
      - deepseek-chat

aliases:
  gpt: openai/gpt-4o
  claude: anthropic/claude-sonnet-4-20250514
```

See [config.example.yaml](config.example.yaml) for a full example.

## Usage

### Interactive mode

```bash
tchat
```

### One-shot mode

```bash
tchat "What is a goroutine?"
tchat "Explain this error" | less
```

## Slash Commands

| Command | Description |
|---------|-------------|
| `/help` | Show available commands |
| `/model [name]` | List or switch models |
| `/config` | Open config in `$EDITOR` |
| `/system [prompt]` | Get/set system prompt |
| `/history` | Show conversation history |
| `/clear` | Clear conversation |
| `/copy` | Copy last response to clipboard |
| `/save [path]` | Export conversation to markdown |
| `/load [path]` | Load a saved conversation |
| `/retry` | Regenerate last response |
| `/temp [value]` | Get/set temperature |
| `/exit` | Quit |

## Keybindings

| Key | Action |
|-----|--------|
| Enter | Send message |
| Shift+Enter | New line |
| Ctrl+X, Ctrl+E | Edit in `$EDITOR` |
| Ctrl+C | Cancel streaming / exit |
| Ctrl+D | Exit |
| Up / Down | Input history |

## License

MIT
