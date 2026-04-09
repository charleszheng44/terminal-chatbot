package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/zc/tchat/internal/provider"
)

// RenderMarkdown renders markdown text using Glamour for terminal display.
// Uses a dark theme, wraps to the given width.
func RenderMarkdown(content string, width int) string {
	if width <= 0 {
		width = 80
	}

	renderer, err := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return content
	}

	rendered, err := renderer.Render(content)
	if err != nil {
		return content
	}

	return strings.TrimRight(rendered, "\n")
}

// FormatMessage formats a single chat message for display.
// User messages get "You:" prefix (bold), assistant messages get rendered as markdown.
func FormatMessage(msg provider.Message, width int) string {
	switch msg.Role {
	case "user":
		return userMsgStyle.Render("You:") + "\n" + msg.Content
	case "assistant":
		return assistantMsgStyle.Render("Assistant:") + "\n" + RenderMarkdown(msg.Content, width)
	case "system":
		return systemMsgStyle.Render(fmt.Sprintf("[system] %s", msg.Content))
	default:
		return msg.Content
	}
}

// FormatMessages formats all messages in history for display in the viewport.
func FormatMessages(msgs []provider.Message, width int) string {
	if len(msgs) == 0 {
		return ""
	}

	var parts []string
	for _, msg := range msgs {
		// Skip system messages from rendering in the viewport (they're internal).
		if msg.Role == "system" {
			continue
		}
		parts = append(parts, FormatMessage(msg, width))
	}
	return strings.Join(parts, "\n\n")
}
