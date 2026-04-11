package command

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/zc/tchat/internal/chat"
	"github.com/zc/tchat/internal/provider"
)

// Config is the interface that the command package expects for configuration.
type Config interface {
	ConfigPath() string
}

// AppState holds shared state that command handlers operate on.
type AppState struct {
	Session   *chat.Session
	Config    Config
	ModelName string
	Quit      func()
	// AvailableModels is a list of model names that can be selected.
	AvailableModels []string
}

// RegisterAll registers all built-in slash commands on the given registry.
func RegisterAll(registry *Registry) {
	registry.Register(Command{
		Name:        "help",
		Description: "Show available commands",
		Handler:     handleHelp,
	})
	registry.Register(Command{
		Name:        "model",
		Description: "List or switch models",
		Handler:     handleModel,
	})
	registry.Register(Command{
		Name:        "config",
		Description: "Open config file in editor",
		Handler:     handleConfig,
	})
	registry.Register(Command{
		Name:        "system",
		Description: "Get or set the system prompt",
		Handler:     handleSystem,
	})
	registry.Register(Command{
		Name:        "history",
		Description: "Show conversation history",
		Handler:     handleHistory,
	})
	registry.Register(Command{
		Name:        "clear",
		Description: "Clear conversation history",
		Handler:     handleClear,
	})
	registry.Register(Command{
		Name:        "copy",
		Description: "Copy last response to clipboard",
		Handler:     handleCopy,
	})
	registry.Register(Command{
		Name:        "save",
		Description: "Export conversation to markdown",
		Handler:     handleSave,
	})
	registry.Register(Command{
		Name:        "load",
		Description: "Load conversation from markdown",
		Handler:     handleLoad,
	})
	registry.Register(Command{
		Name:        "retry",
		Description: "Retry the last message",
		Handler:     handleRetry,
	})
	registry.Register(Command{
		Name:        "temp",
		Description: "Get or set temperature",
		Handler:     handleTemp,
	})
	registry.Register(Command{
		Name:        "exit",
		Description: "Exit tchat",
		Handler:     handleExit,
	})
}

func handleHelp(app *AppState, _ string) (string, error) {
	// We need access to the registry to list commands. Build it inline.
	reg := NewRegistry()
	RegisterAll(reg)
	cmds := reg.All()

	var sb strings.Builder
	sb.WriteString("Available commands:\n")
	for _, cmd := range cmds {
		fmt.Fprintf(&sb, "  /%-10s %s\n", cmd.Name, cmd.Description)
	}
	return sb.String(), nil
}

func handleModel(app *AppState, args string) (string, error) {
	if args == "" {
		if len(app.AvailableModels) == 0 {
			return fmt.Sprintf("Current model: %s\nNo other models configured.", app.ModelName), nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "Current model: %s\n\nAvailable models:\n", app.ModelName)
		for _, m := range app.AvailableModels {
			fmt.Fprintf(&sb, "  - %s\n", m)
		}
		return sb.String(), nil
	}
	return fmt.Sprintf("Switching to model: %s", args), nil
}

func handleConfig(app *AppState, _ string) (string, error) {
	if app.Config == nil {
		return "", fmt.Errorf("no config available")
	}
	return app.Config.ConfigPath(), nil
}

func handleSystem(app *AppState, args string) (string, error) {
	if args == "" {
		prompt := app.Session.SystemPrompt()
		if prompt == "" {
			return "No system prompt set.", nil
		}
		return fmt.Sprintf("Current system prompt:\n%s", prompt), nil
	}
	app.Session.SetSystemPrompt(args)
	return "System prompt updated.", nil
}

func handleHistory(app *AppState, _ string) (string, error) {
	msgs := app.Session.History()
	if len(msgs) == 0 {
		return "No conversation history.", nil
	}

	var sb strings.Builder
	for _, msg := range msgs {
		fmt.Fprintf(&sb, "[%s]\n%s\n\n", msg.Role, msg.Content)
	}
	return sb.String(), nil
}

func handleClear(app *AppState, _ string) (string, error) {
	app.Session.Clear()
	return "Conversation cleared.", nil
}

func handleCopy(app *AppState, _ string) (string, error) {
	msg := app.Session.LastAssistantMessage()
	if msg == "" {
		return "", fmt.Errorf("no assistant message to copy")
	}
	return msg, nil
}

func handleSave(app *AppState, args string) (string, error) {
	path := args
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		ts := time.Now().Format("20060102-150405")
		path = filepath.Join(home, fmt.Sprintf("tchat-export-%s.md", ts))
	}

	msgs := app.Session.History()
	var sb strings.Builder
	sb.WriteString("# tchat conversation\n\n")
	for _, msg := range msgs {
		fmt.Fprintf(&sb, "## %s\n\n%s\n\n", msg.Role, msg.Content)
	}

	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		return "", fmt.Errorf("failed to save: %w", err)
	}
	return fmt.Sprintf("Conversation saved to %s", path), nil
}

func handleLoad(app *AppState, args string) (string, error) {
	if args == "" {
		return "", fmt.Errorf("usage: /load <path>")
	}

	data, err := os.ReadFile(args)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	messages, err := parseMarkdownConversation(string(data))
	if err != nil {
		return "", err
	}

	// Clear existing history and load the parsed messages.
	app.Session.Clear()
	// Remove any existing system prompt so we start fresh.
	app.Session.SetSystemPrompt("")

	loaded := 0
	for _, msg := range messages {
		if msg.Role == "system" {
			app.Session.SetSystemPrompt(msg.Content)
		} else {
			app.Session.AddMessage(msg)
			loaded++
		}
	}

	return fmt.Sprintf("Loaded %d messages from %s", loaded, args), nil
}

// parseMarkdownConversation parses a markdown file exported by /save.
func parseMarkdownConversation(content string) ([]provider.Message, error) {
	var messages []provider.Message
	scanner := bufio.NewScanner(strings.NewReader(content))

	var currentRole string
	var currentContent strings.Builder

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "## ") {
			// Save previous message if any.
			if currentRole != "" {
				messages = append(messages, provider.Message{
					Role:    currentRole,
					Content: strings.TrimSpace(currentContent.String()),
				})
			}
			currentRole = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			currentContent.Reset()
			continue
		}

		// Skip the top-level title line.
		if strings.HasPrefix(line, "# ") {
			continue
		}

		if currentRole != "" {
			currentContent.WriteString(line)
			currentContent.WriteString("\n")
		}
	}

	// Don't forget the last message.
	if currentRole != "" {
		messages = append(messages, provider.Message{
			Role:    currentRole,
			Content: strings.TrimSpace(currentContent.String()),
		})
	}

	return messages, nil
}

func handleRetry(app *AppState, _ string) (string, error) {
	return app.Session.Retry(context.Background(), nil)
}

func handleTemp(app *AppState, args string) (string, error) {
	if args == "" {
		return fmt.Sprintf("Current temperature: %.2f", app.Session.Temperature()), nil
	}

	t, err := strconv.ParseFloat(strings.TrimSpace(args), 64)
	if err != nil {
		return "", fmt.Errorf("invalid temperature value: %s", args)
	}
	app.Session.SetTemperature(t)
	return fmt.Sprintf("Temperature set to %.2f", t), nil
}

func handleExit(app *AppState, _ string) (string, error) {
	if app.Quit != nil {
		app.Quit()
	}
	return "", nil
}
