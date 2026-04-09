package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/zc/tchat/internal/config"
	"github.com/zc/tchat/internal/oneshot"
	"github.com/zc/tchat/internal/ui"
)

var rootCmd = &cobra.Command{
	Use:   "tchat [prompt]",
	Short: "Terminal chatbot for multiple LLM providers",
	Long:  "A terminal-based chatbot that provides a unified interface to OpenAI, Anthropic, and Google Gemini.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			// If config not found, print a helpful message rather than crashing.
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
			fmt.Fprintf(os.Stderr, "Create a config file at %s to get started.\n", config.ConfigPath())
			fmt.Fprintf(os.Stderr, "See config.example.yaml for reference.\n")
			return nil
		}

		if len(args) == 1 {
			return oneshot.Run(cfg, args[0])
		}

		// Interactive mode.
		model, err := ui.New(cfg)
		if err != nil {
			return fmt.Errorf("initialize TUI: %w", err)
		}

		p := tea.NewProgram(model, tea.WithAltScreen())
		model.SetProgram(p)

		if _, err := p.Run(); err != nil {
			return fmt.Errorf("run TUI: %w", err)
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
