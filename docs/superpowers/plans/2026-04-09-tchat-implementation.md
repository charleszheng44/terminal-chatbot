# tchat Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a terminal-based multi-provider LLM chatbot CLI called `tchat`

**Architecture:** Go CLI using Cobra for command parsing, Bubble Tea for TUI, with a provider abstraction layer over OpenAI/Anthropic/Gemini APIs. Config via YAML file.

**Tech Stack:** Go, Bubble Tea, Lip Gloss, Glamour, Cobra, go-openai, anthropic-sdk-go, generative-ai-go

---

### Task 1: Project Scaffold & CLI Entrypoint

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `cmd/root.go`

- [ ] **Step 1: Initialize Go module**

Run: `go mod init github.com/zc/tchat`

- [ ] **Step 2: Install Cobra**

Run: `go get github.com/spf13/cobra@latest`

- [ ] **Step 3: Create `main.go`**

```go
package main

import "github.com/zc/tchat/cmd"

func main() {
	cmd.Execute()
}
```

- [ ] **Step 4: Create `cmd/root.go`**

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "tchat [prompt]",
	Short: "Terminal chatbot for multiple LLM providers",
	Long:  "A terminal-based chatbot that provides a unified interface to OpenAI, Anthropic, and Google Gemini.",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 1 {
			fmt.Println("One-shot mode:", args[0])
		} else {
			fmt.Println("Interactive mode (TUI coming soon)")
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
```

- [ ] **Step 5: Build and test**

Run: `go build -o tchat . && ./tchat && ./tchat "hello"`

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum main.go cmd/
git commit -m "feat: project scaffold with cobra CLI entrypoint"
```

---

### Task 2: Config Package

**Files:**
- Create: `internal/config/config.go`
- Create: `config.example.yaml`

- [ ] **Step 1: Install YAML dependency**

Run: `go get gopkg.in/yaml.v3`

- [ ] **Step 2: Create `internal/config/config.go`**

Config struct definitions, loading from `~/.config/tchat/config.yaml`, defaults, alias resolution, model validation.

- [ ] **Step 3: Create `config.example.yaml`**

Example config with all providers and aliases documented.

- [ ] **Step 4: Build and verify**

Run: `go build ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/config/ config.example.yaml
git commit -m "feat: config package with YAML loading and model aliases"
```

---

### Task 3: Provider Interface & Types

**Files:**
- Create: `internal/provider/provider.go`

- [ ] **Step 1: Create `internal/provider/provider.go`**

Define `Provider` interface, `Message`, `ChatOptions` types, and `NewProvider` factory function that dispatches to the correct implementation based on provider name.

- [ ] **Step 2: Build and verify**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/provider/provider.go
git commit -m "feat: provider interface and shared types"
```

---

### Task 4: OpenAI Provider

**Files:**
- Create: `internal/provider/openai.go`

- [ ] **Step 1: Install go-openai SDK**

Run: `go get github.com/sashabaranov/go-openai`

- [ ] **Step 2: Create `internal/provider/openai.go`**

Implement `Provider` interface: `Chat`, `StreamChat`, `ListModels`. Support custom `base_url` for OpenAI-compatible services.

- [ ] **Step 3: Build and verify**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/provider/openai.go go.mod go.sum
git commit -m "feat: OpenAI provider with streaming and custom base URL support"
```

---

### Task 5: Anthropic Provider

**Files:**
- Create: `internal/provider/anthropic.go`

- [ ] **Step 1: Install anthropic SDK**

Run: `go get github.com/anthropics/anthropic-sdk-go`

- [ ] **Step 2: Create `internal/provider/anthropic.go`**

Implement `Provider` interface for Anthropic. Map `Message` to Anthropic's format (separate system message handling). Handle streaming via Anthropic's SSE event types.

- [ ] **Step 3: Build and verify**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/provider/anthropic.go go.mod go.sum
git commit -m "feat: Anthropic provider with streaming support"
```

---

### Task 6: Gemini Provider

**Files:**
- Create: `internal/provider/gemini.go`

- [ ] **Step 1: Install Gemini SDK**

Run: `go get github.com/google/generative-ai-go`
Run: `go get google.golang.org/api`

- [ ] **Step 2: Create `internal/provider/gemini.go`**

Implement `Provider` interface for Gemini. Map roles (user/assistant → user/model). Handle streaming via Gemini's GenerateContentStream.

- [ ] **Step 3: Build and verify**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/provider/gemini.go go.mod go.sum
git commit -m "feat: Gemini provider with streaming support"
```

---

### Task 7: Chat Manager

**Files:**
- Create: `internal/chat/chat.go`

- [ ] **Step 1: Create `internal/chat/chat.go`**

`Session` struct managing conversation state: message history, active provider, system prompt, temperature. Methods: `Send` (streaming + non-streaming), `SetSystemPrompt`, `SetTemperature`, `Clear`, `History`, `Retry` (remove last exchange and resend).

- [ ] **Step 2: Build and verify**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/chat/
git commit -m "feat: chat session manager with conversation state"
```

---

### Task 8: Command Registry

**Files:**
- Create: `internal/command/command.go`
- Create: `internal/command/commands.go`

- [ ] **Step 1: Create `internal/command/command.go`**

`Registry` struct, `Command` type with Name/Description/Handler, `Parse` function to detect and dispatch slash commands.

- [ ] **Step 2: Create `internal/command/commands.go`**

Register all 12 commands: `/help`, `/model`, `/config`, `/system`, `/history`, `/clear`, `/copy`, `/save`, `/load`, `/retry`, `/temp`, `/exit`. Each handler receives app state and args string.

- [ ] **Step 3: Build and verify**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/command/
git commit -m "feat: slash command registry with all 12 commands"
```

---

### Task 9: Basic TUI Shell

**Files:**
- Create: `internal/ui/model.go`
- Create: `internal/ui/view.go`
- Create: `internal/ui/input.go`
- Create: `internal/ui/styles.go`
- Modify: `cmd/root.go`

- [ ] **Step 1: Install Bubble Tea, Lip Gloss, Glamour**

Run: `go get github.com/charmbracelet/bubbletea@latest github.com/charmbracelet/lipgloss@latest github.com/charmbracelet/glamour@latest github.com/charmbracelet/bubbles@latest`

- [ ] **Step 2: Create `internal/ui/styles.go`**

Lip Gloss styles for status bar, user messages, assistant messages, input area.

- [ ] **Step 3: Create `internal/ui/input.go`**

Input component: multi-line textarea with Enter to send, Shift+Enter for newline, input history (Up/Down), Ctrl+X Ctrl+E chord for external editor.

- [ ] **Step 4: Create `internal/ui/view.go`**

View rendering: status bar, chat viewport, input area. Glamour markdown rendering for assistant messages.

- [ ] **Step 5: Create `internal/ui/model.go`**

Main Bubble Tea model: `Init`, `Update`, `View`. Wires together config, chat session, command registry, input, viewport. Handles streaming messages, window resize.

- [ ] **Step 6: Update `cmd/root.go`**

Wire interactive mode to launch TUI, one-shot mode to run without TUI.

- [ ] **Step 7: Build and test interactively**

Run: `go build -o tchat . && ./tchat`

- [ ] **Step 8: Commit**

```bash
git add internal/ui/ cmd/root.go go.mod go.sum
git commit -m "feat: Bubble Tea TUI with markdown rendering and input handling"
```

---

### Task 10: Streaming Integration

**Files:**
- Modify: `internal/ui/model.go`

- [ ] **Step 1: Add streaming message types and update handler**

Define `StreamTokenMsg`, `StreamDoneMsg`, `StreamErrorMsg`. In `Update`, handle token accumulation, debounced Glamour re-rendering (~50ms tick), and final render on done. Show spinner during generation.

- [ ] **Step 2: Wire Send to provider streaming**

When user sends a message, start a `tea.Cmd` that calls `chat.Send()` with an `onToken` callback that sends `StreamTokenMsg` back to the Bubble Tea program.

- [ ] **Step 3: Non-streaming fallback**

When streaming is disabled, show a spinner, call `Chat()`, render full response on completion.

- [ ] **Step 4: Build and test with a real provider**

Run: `go build -o tchat . && ./tchat`

- [ ] **Step 5: Commit**

```bash
git add internal/ui/
git commit -m "feat: streaming response rendering with debounced markdown"
```

---

### Task 11: One-Shot Mode

**Files:**
- Create: `internal/oneshot/oneshot.go`
- Modify: `cmd/root.go`

- [ ] **Step 1: Create `internal/oneshot/oneshot.go`**

`Run` function: load config, create provider, send single message, stream to stdout. If stdout is a TTY, render markdown via Glamour. If piped, output plain text.

- [ ] **Step 2: Update `cmd/root.go`**

When args has a prompt, call `oneshot.Run` instead of launching TUI.

- [ ] **Step 3: Build and test**

Run: `go build -o tchat . && ./tchat "What is Go?" && ./tchat "What is Go?" | cat`

- [ ] **Step 4: Commit**

```bash
git add internal/oneshot/ cmd/root.go
git commit -m "feat: one-shot mode with TTY-aware markdown rendering"
```

---

### Task 12: Polish & Config Example

**Files:**
- Modify: `config.example.yaml`
- Create: `.gitignore`

- [ ] **Step 1: Create `.gitignore`**

Ignore `tchat` binary, `.env`, etc.

- [ ] **Step 2: Finalize `config.example.yaml`**

Ensure example config matches all implemented features with comments.

- [ ] **Step 3: Build final binary and smoke test**

Run: `go build -o tchat . && ./tchat`

- [ ] **Step 4: Commit**

```bash
git add .gitignore config.example.yaml
git commit -m "chore: add gitignore and finalize example config"
```
