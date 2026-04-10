package ui

import "github.com/charmbracelet/lipgloss"

var (
	// statusBarStyle is a full-width bar with inverted colors (light on dark).
	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)

	// userMsgStyle renders user messages with a bold "You:" prefix.
	userMsgStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("36"))

	// assistantMsgStyle renders assistant messages with an "Assistant:" prefix.
	assistantMsgStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("212"))

	// systemMsgStyle renders system/info messages in a dimmed italic style.
	systemMsgStyle = lipgloss.NewStyle().
			Faint(true).
			Italic(true).
			Foreground(lipgloss.Color("245"))

	// inputPromptStyle renders the "❯ " prompt indicator.
	inputPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("69")).
				Bold(true)

	// inputBoxStyle frames the input area with horizontal rules above and
	// below (no side borders), matching Claude Code's input look.
	inputBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, true, false).
			BorderForeground(lipgloss.Color("240"))

	// errorStyle renders error text in bold red.
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	// spinnerStyle renders the streaming/loading indicator.
	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("69"))
)
