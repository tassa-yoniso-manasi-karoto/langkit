package cli

import (
	"fmt"
	"os"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/cli/commands"
)

func Run() {
	// Execute adds all child commands to the root command and sets flags appropriately.
	if err := commands.RootCmd.Execute(); err != nil {
		fmt.Printf("rootCmd error: %v\n", err)
		os.Exit(1)
	}
}