package main

import (
	"os"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/cli"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/gui"
)

func main() {
	if len(os.Args) > 1 {
		if os.Args[1] == "--server" {
			gui.RunServerMode()
		} else {
			cli.Run()
		}
	} else {
		gui.Run()
	}
}