package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/zc/tchat/internal/chat"
	"github.com/zc/tchat/internal/command"
	"github.com/zc/tchat/internal/config"
	"github.com/zc/tchat/internal/provider"
)

// Message types for streaming.
type StreamTokenMsg struct{ Token string }
type StreamDoneMsg struct{ FullResponse string }
type StreamErrorMsg struct{ Err error }
type renderTickMsg struct{}

// configAdapter wraps *config.Config to satisfy command.Config interface.
type configAdapter struct {
	cfg *config.Config
}

func (a *configAdapter) ConfigPath() string {
	return config.ConfigPath()
}

// Model is the main Bubble Tea model for the TUI.
type Model struct {
	cfg        *config.Config
	session    *chat.Session
	registry   *command.Registry
	appState   *command.AppState
	input      InputModel
	viewport   viewport.Model
	spinner    spinner.Model
	modelName  string
	streaming  bool
	streamBuf  strings.Builder
	messages   string
	width      int
	height     int
	err        error
	quitting   bool
	program    *tea.Program
	cancel     context.CancelFunc
	ticking    bool
	pickerMode bool
	picker     *ModelPicker
	// notification is a transient UI message (e.g. /help output,
	// "Switched to X", "Copied to clipboard.") that lives outside of
	// session history. It is rendered below the history on every
	// updateViewportContent call and cleared when the user starts a new
	// chat turn.
	notification string
}

// SetProgram stores the tea.Program reference on the model so that streaming
// commands can send messages back into the event loop.
func (m *Model) SetProgram(p *tea.Program) {
	m.program = p
}

// New creates a new Model from the given config.
func New(cfg *config.Config) (*Model, error) {
	// Resolve the default model.
	provName, modelName, err := cfg.ResolveModel(cfg.DefaultModel)
	if err != nil {
		return nil, fmt.Errorf("resolve default model: %w", err)
	}

	provCfg, ok := cfg.Providers[provName]
	if !ok {
		return nil, fmt.Errorf("provider %q not found in config", provName)
	}

	p, err := provider.NewProvider(provName, provCfg)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}

	opts := provider.ChatOptions{
		Model:       modelName,
		Temperature: cfg.Defaults.Temperature,
		MaxTokens:   cfg.Defaults.MaxTokens,
	}

	session := chat.NewSession(p, opts)

	// Set system prompt from config if non-empty.
	if cfg.Defaults.SystemPrompt != "" {
		session.SetSystemPrompt(cfg.Defaults.SystemPrompt)
	}

	// Build available models list.
	var availableModels []string
	for pn, pc := range cfg.Providers {
		for _, m := range pc.Models {
			availableModels = append(availableModels, pn+"/"+m)
		}
	}

	// Set up command registry.
	registry := command.NewRegistry()
	command.RegisterAll(registry)

	displayName := provName + "/" + modelName

	// Initialize spinner.
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = spinnerStyle

	// Initialize viewport.
	vp := viewport.New(80, 20)

	// Initialize input.
	input := NewInputModel()

	m := &Model{
		cfg:       cfg,
		session:   session,
		registry:  registry,
		input:     input,
		viewport:  vp,
		spinner:   sp,
		modelName: displayName,
	}

	// Build app state.
	appState := &command.AppState{
		Session:         session,
		Config:          &configAdapter{cfg: cfg},
		ModelName:       displayName,
		AvailableModels: availableModels,
		Quit: func() {
			if m.program != nil {
				m.program.Send(tea.Quit())
			}
		},
	}
	m.appState = appState

	return m, nil
}

// Init returns the initial commands for the model.
func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.input.Init())
}

// Update handles messages for the model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// While the model picker is open, route keys to it exclusively and
	// drop every other message. The picker is modal — nothing else should
	// update while it's up.
	if m.pickerMode {
		if km, ok := msg.(tea.KeyMsg); ok {
			return m.handlePickerKey(km)
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcLayout()
		return m, nil

	case SubmitMsg:
		return m.handleSubmit(msg.Text)

	case StreamTokenMsg:
		m.streamBuf.WriteString(msg.Token)
		if !m.ticking {
			m.ticking = true
			cmds = append(cmds, m.renderTick())
		}
		return m, tea.Batch(cmds...)

	case renderTickMsg:
		m.ticking = false
		m.updateViewportContent(true)
		return m, nil

	case StreamDoneMsg:
		m.streaming = false
		m.ticking = false
		m.cancel = nil
		m.streamBuf.Reset()
		m.updateViewportContent(false)
		return m, nil

	case StreamErrorMsg:
		m.streaming = false
		m.ticking = false
		m.cancel = nil
		m.streamBuf.Reset()
		m.err = msg.Err
		m.updateViewportContent(false)
		return m, nil

	case EditorRequestMsg:
		return m.handleEditorRequest()

	case editorResultMsg:
		m.input.SetValue(msg.text)
		return m, nil

	case spinner.TickMsg:
		if m.streaming {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.streaming && m.cancel != nil {
				m.cancel()
				m.streaming = false
				m.ticking = false
				m.streamBuf.Reset()
				m.updateViewportContent(false)
				return m, nil
			}
			m.quitting = true
			return m, tea.Quit

		case "ctrl+d":
			m.quitting = true
			return m, tea.Quit
		}
	}

	// Pass through to input.
	var inputCmd tea.Cmd
	m.input, inputCmd = m.input.Update(msg)
	if inputCmd != nil {
		cmds = append(cmds, inputCmd)
	}

	// Pass through to viewport for scrolling.
	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	if vpCmd != nil {
		cmds = append(cmds, vpCmd)
	}

	return m, tea.Batch(cmds...)
}

// handleSubmit processes submitted user input (slash commands or chat messages).
func (m *Model) handleSubmit(text string) (tea.Model, tea.Cmd) {
	cmdName, args, isCommand := command.Parse(text)

	if isCommand {
		cmd := m.registry.Get(cmdName)
		if cmd == nil {
			m.err = fmt.Errorf("unknown command: /%s", cmdName)
			m.updateViewportContent(false)
			return m, nil
		}

		// Special handling for /model.
		//   /model <name>  → direct switch (unchanged)
		//   /model         → interactive picker (arrow keys to select)
		if cmdName == "model" {
			if args != "" {
				return m.handleModelSwitch(args)
			}
			return m.openModelPicker()
		}

		// Special handling for /config — open editor.
		if cmdName == "config" {
			result, err := cmd.Handler(m.appState, args)
			if err != nil {
				m.err = err
				m.updateViewportContent(false)
				return m, nil
			}
			// result is the config path.
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			c := exec.Command(editor, result)
			return m, tea.ExecProcess(c, func(err error) tea.Msg {
				if err != nil {
					return StreamErrorMsg{Err: fmt.Errorf("editor: %w", err)}
				}
				return nil
			})
		}

		// Special handling for /copy — copy to clipboard.
		if cmdName == "copy" {
			result, err := cmd.Handler(m.appState, args)
			if err != nil {
				m.err = err
				m.updateViewportContent(false)
				return m, nil
			}
			if clipErr := clipboard.WriteAll(result); clipErr != nil {
				m.err = fmt.Errorf("clipboard: %w", clipErr)
				m.updateViewportContent(false)
				return m, nil
			}
			m.addSystemMessage("Copied to clipboard.")
			m.updateViewportContent(false)
			return m, nil
		}

		// Special handling for /retry — start streaming.
		if cmdName == "retry" {
			return m.handleRetry()
		}

		// Generic command execution.
		result, err := cmd.Handler(m.appState, args)
		if err != nil {
			m.err = err
			m.updateViewportContent(false)
			return m, nil
		}
		if result != "" {
			m.addSystemMessage(result)
			m.updateViewportContent(false)
		}
		return m, nil
	}

	// Regular chat message — start streaming.
	m.err = nil
	m.notification = "" // clear any stale /help or "Switched to X" banner
	m.streaming = true
	return m, tea.Batch(m.sendMessage(text), m.spinner.Tick)
}

// openModelPicker enters modal picker mode for /model with no args.
// Falls back to a text message when there are no models to pick from.
func (m *Model) openModelPicker() (tea.Model, tea.Cmd) {
	if len(m.appState.AvailableModels) == 0 {
		m.addSystemMessage(fmt.Sprintf("Current model: %s\nNo other models configured.", m.modelName))
		m.updateViewportContent(false)
		return m, nil
	}
	m.picker = NewModelPicker(m.appState.AvailableModels, m.modelName)
	m.pickerMode = true
	return m, nil
}

// handlePickerKey processes a key press while the model picker is open.
func (m *Model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		m.picker.MoveUp()
		return m, nil
	case "down":
		m.picker.MoveDown()
		return m, nil
	case "enter":
		selected := m.picker.Selected()
		m.pickerMode = false
		m.picker = nil
		if selected == "" || selected == m.modelName {
			return m, nil
		}
		return m.handleModelSwitch(selected)
	case "esc", "ctrl+c":
		m.pickerMode = false
		m.picker = nil
		return m, nil
	}
	return m, nil
}

// handleModelSwitch switches to a new model.
func (m *Model) handleModelSwitch(modelSpec string) (tea.Model, tea.Cmd) {
	provName, modelName, err := m.cfg.ResolveModel(modelSpec)
	if err != nil {
		m.err = fmt.Errorf("resolve model: %w", err)
		m.updateViewportContent(false)
		return m, nil
	}

	provCfg, ok := m.cfg.Providers[provName]
	if !ok {
		m.err = fmt.Errorf("provider %q not found", provName)
		m.updateViewportContent(false)
		return m, nil
	}

	p, err := provider.NewProvider(provName, provCfg)
	if err != nil {
		m.err = fmt.Errorf("create provider: %w", err)
		m.updateViewportContent(false)
		return m, nil
	}

	m.session.SetProvider(p)
	m.session.SetModel(modelName)
	m.modelName = provName + "/" + modelName
	m.appState.ModelName = m.modelName
	m.addSystemMessage(fmt.Sprintf("Switched to %s", m.modelName))
	m.updateViewportContent(false)
	return m, nil
}

// handleRetry retries the last message.
func (m *Model) handleRetry() (tea.Model, tea.Cmd) {
	m.err = nil
	m.streaming = true

	retryCmd := func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		m.cancel = cancel

		_, err := m.session.Retry(ctx, func(token string) {
			if m.program != nil {
				m.program.Send(StreamTokenMsg{Token: token})
			}
		})
		if err != nil {
			return StreamErrorMsg{Err: err}
		}
		return StreamDoneMsg{}
	}

	return m, tea.Batch(tea.Cmd(retryCmd), m.spinner.Tick)
}

// handleEditorRequest opens $EDITOR with the current input in a temp file.
func (m *Model) handleEditorRequest() (tea.Model, tea.Cmd) {
	currentText := m.input.Value()

	tmpFile, err := os.CreateTemp("", "tchat-input-*.md")
	if err != nil {
		m.err = fmt.Errorf("create temp file: %w", err)
		return m, nil
	}

	if _, err := tmpFile.WriteString(currentText); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		m.err = fmt.Errorf("write temp file: %w", err)
		return m, nil
	}
	_ = tmpFile.Close()

	tmpPath := tmpFile.Name()
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	c := exec.Command(editor, tmpPath)
	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		defer func() { _ = os.Remove(tmpPath) }()
		if err != nil {
			return StreamErrorMsg{Err: fmt.Errorf("editor: %w", err)}
		}
		data, readErr := os.ReadFile(tmpPath)
		if readErr != nil {
			return StreamErrorMsg{Err: fmt.Errorf("read temp file: %w", readErr)}
		}
		return editorResultMsg{text: strings.TrimRight(string(data), "\n")}
	})
}

// editorResultMsg carries text back from an external editor session.
type editorResultMsg struct {
	text string
}

// sendMessage creates a tea.Cmd that sends a chat message with streaming.
func (m *Model) sendMessage(input string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		m.cancel = cancel

		_, err := m.session.Send(ctx, input, func(token string) {
			if m.program != nil {
				m.program.Send(StreamTokenMsg{Token: token})
			}
		})
		if err != nil {
			return StreamErrorMsg{Err: err}
		}
		return StreamDoneMsg{}
	}
}

// renderTick returns a command that sends a renderTickMsg after a short delay.
func (m *Model) renderTick() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(_ time.Time) tea.Msg {
		return renderTickMsg{}
	})
}

// addSystemMessage queues a transient UI notification to be rendered
// below the chat history on the next updateViewportContent call. The
// notification persists across intermediate re-renders (resize,
// streaming ticks, picker toggle) and is cleared when the user starts
// a new chat turn in handleSubmit.
//
// Calling this again replaces the prior notification rather than
// appending, which is what we want for the current call sites
// ("Switched to X", "Copied to clipboard.", /help output, etc.).
func (m *Model) addSystemMessage(text string) {
	m.notification = text
}

// updateViewportContent re-renders the chat content and updates the viewport.
func (m *Model) updateViewportContent(withStream bool) {
	// Re-render all messages from session history.
	msgs := m.session.History()
	m.messages = FormatMessages(msgs, m.viewportWidth())

	// If we have error, append it.
	if m.err != nil {
		errMsg := errorStyle.Render("Error: " + m.err.Error())
		if m.messages != "" {
			m.messages += "\n\n" + errMsg
		} else {
			m.messages = errMsg
		}
	}

	// If streaming, append the partial response.
	if withStream && m.streamBuf.Len() > 0 {
		partial := assistantMsgStyle.Render("Assistant:") + "\n" +
			RenderMarkdown(m.streamBuf.String(), m.viewportWidth())
		if m.messages != "" {
			m.messages += "\n\n" + partial
		} else {
			m.messages = partial
		}
	}

	// Append any pending UI notification last so it renders below the
	// history, errors, and streaming partial.
	if m.notification != "" {
		note := systemMsgStyle.Render(m.notification)
		if m.messages != "" {
			m.messages += "\n\n" + note
		} else {
			m.messages = note
		}
	}

	m.viewport.SetContent(m.messages)
	m.viewport.GotoBottom()
}

// viewportWidth returns the usable width for the viewport content.
func (m *Model) viewportWidth() int {
	w := m.width - 2
	if w < 40 {
		w = 40
	}
	return w
}

// recalcLayout recalculates viewport and input dimensions after a resize.
func (m *Model) recalcLayout() {
	statusBarHeight := 1
	inputHeight := m.input.textarea.Height() + 2 // +2 for top and bottom border rules
	vpHeight := m.height - statusBarHeight - inputHeight
	if vpHeight < 1 {
		vpHeight = 1
	}

	m.viewport.Width = m.width
	m.viewport.Height = vpHeight
	m.input.SetWidth(m.width)
	m.updateViewportContent(m.streaming)
}

// View renders the full TUI.
func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	// Status bar.
	statusText := fmt.Sprintf("tchat \u2014 %s", m.modelName)
	if m.streaming {
		statusText += "     " + m.spinner.View()
	}
	statusBar := statusBarStyle.
		Width(m.width).
		Render(statusText)

	// When the picker is open, render it in place of the viewport. The
	// input and status bar stay visible but inert.
	mainContent := m.viewport.View()
	if m.pickerMode && m.picker != nil {
		mainContent = m.picker.View(m.width, m.viewport.Height)
	}

	// Build the full view.
	return lipgloss.JoinVertical(
		lipgloss.Left,
		statusBar,
		mainContent,
		m.input.View(),
	)
}
