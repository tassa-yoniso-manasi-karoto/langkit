package commands

import (
	"fmt"
	"context"
	
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	
	"github.com/spf13/cobra"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
)

var translitCmd = &cobra.Command{
	Use:   "translit <foreign-subs>",
	Short: "transliterate and tokenize a subtitle file",

	Args: argFuncs(cobra.MinimumNArgs(1), cobra.MaximumNArgs(1)),
	Run: RunWithExit(translit),
}

func translit(tsk *core.Task, ctx context.Context, cmd *cobra.Command, args []string) *core.ProcessingError {
	if len(args) == 0 {
		return tsk.Handler.Log(core.Error, "", "this command requires one argument: the path to the subtitle file to be processed")
	}
	tsk.ApplyFlags(cmd)
	tsk.TargSubFile = args[0]
	
	tsk.WantTranslit = true
	
	tsk.Mode = core.Translit
	
	return tsk.Routing(ctx)
}

func placeholder2345432() {
	fmt.Println("")
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
