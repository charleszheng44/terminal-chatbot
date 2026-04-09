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
