package commands

import (
	"fmt"
	
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	
	"github.com/spf13/cobra"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/core"
)

var translitCmd = &cobra.Command{
	Use:   "translit <foreign-subs>",
	Short: "transliterate and tokenize a subtitle file",

	Args: argFuncs(cobra.MinimumNArgs(1), cobra.MaximumNArgs(1)),
	Run: func(cmd *cobra.Command, args []string) {
		tsk := core.NewTask(core.NewCLIHandler())
		if len(args) == 0 {
			tsk.Handler.ZeroLog().Fatal().Msg("this command requires one argument: the path to the subtitle file to be processed")
		}
		tsk.ApplyFlags(cmd)
		tsk.TargSubFile = args[0]
		
		tsk.WantTranslit = true
		
		tsk.Mode = core.Translit
		tsk.Execute() // FIXME or routing??
	},
}


func placeholder2345432() {
	fmt.Println("")
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}
