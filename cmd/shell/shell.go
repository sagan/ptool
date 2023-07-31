package shell

import (
	"fmt"

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
	DynamicSuggestionsFunc: cmd.ShellDynamicSuggestionsFunc,
	OnErrorFunc: func(err error) {
		cmd.RootCmd.PrintErrln(err)
	},
}

func shell(command *cobra.Command, args []string) {
	if config.Fork || config.LockFile != "" {
		log.Fatalf("--fork or --lock flag can NOT be used with shell")
	}
	cmd.RootCmd.AddCommand(ShellCommands...)
	for name := range ShellCommandSuggestions {
		cmd.AddShellCompletion(name, ShellCommandSuggestions[name])
	}
	if !config.Get().Hushshell {
		fmt.Printf("Welcome to ptool shell\n")
		fmt.Printf("In addition to normal ptool commands, you can use the shell commands here:\n")
		for _, shellCmd := range ShellCommands {
			fmt.Printf("* %s : %s\n", shellCmd.Name(), shellCmd.Short)
		}
		fmt.Printf(`Type "<command> -h" to see full help` + "\n")
		fmt.Printf(`Note client data will be cached in shell, run "purge [client]..." to purge cache` + "\n")
		fmt.Printf(`Use "exit" or Ctrl + D to exit shell` + "\n")
		fmt.Printf(`To mute this message, add "hushshell = true" line to the top of ptool.toml config file` + "\n")
	}
	advancedPrompt.Run()
}
