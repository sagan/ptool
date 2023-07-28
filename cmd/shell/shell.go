package shell

import (
	"github.com/c-bata/go-prompt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	cobraprompt "github.com/stromland/cobra-prompt"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
)

var command = &cobra.Command{
	Use:   "shell",
	Short: "Start a interactive shell in which you can execute any ptool commands.",
	Long:  `Start a interactive shell in which you can execute any ptool commands.`,
	Args:  cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
	Run:   shell,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

var advancedPrompt = &cobraprompt.CobraPrompt{
	RootCmd:                  cmd.RootCmd,
	PersistFlagValues:        true,
	ShowHelpCommandAndFlags:  true,
	DisableCompletionCommand: true,
	GoPromptOptions: []prompt.Option{
		prompt.OptionTitle("ptool-shell"),
		prompt.OptionPrefix("> "),
		prompt.OptionMaxSuggestion(5),
	},
	OnErrorFunc: func(err error) {
		cmd.RootCmd.PrintErrln(err)
	},
}

func shell(command *cobra.Command, args []string) {
	if config.Fork || config.LockFile != "" {
		log.Fatalf("--fork or --lock flag can NOT be used with shell")
	}
	cmd.RootCmd.AddCommand(Pwd)
	cmd.RootCmd.AddCommand(Cd)
	cmd.RootCmd.AddCommand(Exec)
	cmd.RootCmd.AddCommand(Ls)
	cmd.RootCmd.AddCommand(Exit)
	advancedPrompt.Run()
}
