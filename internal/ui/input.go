package ui

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SubmitMsg is sent when the user presses Enter to submit input.
type SubmitMsg struct {
	Text string
}

// EditorRequestMsg is sent when the user presses Ctrl+X followed by Ctrl+E.
type EditorRequestMsg struct{}

const maxInputHeight = 5

// InputModel wraps a textarea.Model for multi-line chat input.
type InputModel struct {
	textarea     textarea.Model
	history      []string
	historyIndex int
	ctrlXPressed bool
	width        int
}

// NewInputModel creates a new InputModel with sensible defaults.
func NewInputModel() InputModel {
	ta := textarea.New()
	ta.Prompt = inputPromptStyle.Render("❯ ")
	ta.CharLimit = 0 // unlimited
	ta.MaxHeight = maxInputHeight
	ta.ShowLineNumbers = false
	ta.SetHeight(1)

	// The bubbles textarea defaults give CursorLine a background color, which
	// renders as a grey band on the cursor's line. Clear it on both focused
	// and blurred states so the input has no background tint.
	noBg := lipgloss.NewStyle()
	ta.FocusedStyle.CursorLine = noBg
	ta.BlurredStyle.CursorLine = noBg

	ta.Focus()

	// Remove default keybindings for Enter so we can handle it ourselves.
	ta.KeyMap.InsertNewline.SetEnabled(false)

	return InputModel{
		textarea:     ta,
		historyIndex: -1,
	}
}

// Init satisfies the tea.Model interface.
func (m InputModel) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles key messages and delegates to the inner textarea.
func (m InputModel) Update(msg tea.Msg) (InputModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Ctrl+X / Ctrl+E chord detection.
		if m.ctrlXPressed {
			m.ctrlXPressed = false
			if msg.String() == "ctrl+e" {
				return m, func() tea.Msg { return EditorRequestMsg{} }
			}
			// Not Ctrl+E — fall through to normal handling.
		}

		switch msg.String() {
		case "ctrl+x":
			m.ctrlXPressed = true
			return m, nil

		case "enter":
			text := m.textarea.Value()
			if text == "" {
				return m, nil
			}
			// Append to history and reset index.
			m.history = append(m.history, text)
			m.historyIndex = -1
			m.Reset()
			return m, func() tea.Msg { return SubmitMsg{Text: text} }

		case "shift+enter":
			// Insert a newline character.
			m.textarea, cmd = m.textarea.Update(tea.KeyMsg{
				Type:  tea.KeyRunes,
				Runes: []rune{'\n'},
			})
			m.resizeHeight()
			return m, cmd

		case "up":
			if m.textarea.Value() == "" && len(m.history) > 0 {
				if m.historyIndex == -1 {
					m.historyIndex = len(m.history) - 1
				} else if m.historyIndex > 0 {
					m.historyIndex--
				}
				m.textarea.SetValue(m.history[m.historyIndex])
				m.textarea.CursorEnd()
				return m, nil
			}

		case "down":
			if m.historyIndex >= 0 {
				if m.historyIndex < len(m.history)-1 {
					m.historyIndex++
					m.textarea.SetValue(m.history[m.historyIndex])
					m.textarea.CursorEnd()
				} else {
					m.historyIndex = -1
					m.textarea.SetValue("")
				}
				return m, nil
			}
		}
	}

	m.textarea, cmd = m.textarea.Update(msg)
	m.resizeHeight()
	return m, cmd
}

// View renders the input component, wrapped in the input box frame
// (horizontal rules above and below).
func (m InputModel) View() string {
	ta := m.textarea.View()
	if m.width <= 0 {
		// Before the first WindowSizeMsg we don't know the terminal width;
		// fall back to the bare textarea so we don't render a stubby border.
		return ta
	}
	return inputBoxStyle.Width(m.width).Render(ta)
}

// Value returns the current input text.
func (m InputModel) Value() string {
	return m.textarea.Value()
}

// Reset clears the input and resets height.
func (m *InputModel) Reset() {
	m.textarea.Reset()
	m.textarea.SetHeight(1)
}

// SetValue sets the input text.
func (m *InputModel) SetValue(s string) {
	m.textarea.SetValue(s)
	m.resizeHeight()
}

// SetWidth sets the input width. The textarea itself uses the full width
// (no side borders), so it receives the same value as the outer box.
func (m *InputModel) SetWidth(w int) {
	m.width = w
	m.textarea.SetWidth(w)
}

// resizeHeight adjusts the textarea height based on content, up to maxInputHeight.
func (m *InputModel) resizeHeight() {
	lines := m.textarea.LineCount()
	if lines < 1 {
		lines = 1
	}
	if lines > maxInputHeight {
		lines = maxInputHeight
	}
	m.textarea.SetHeight(lines)
}
