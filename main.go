package main

import (
	"os"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/cli"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/gui"
)

func main() {
	if len(os.Args) > 1 {
		cli.Run()
	} else {
		gui.Run()
	}
}