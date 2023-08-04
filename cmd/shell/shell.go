package shell

import (
	"fmt"

	"github.com/c-bata/go-prompt"
	"github.com/google/shlex"
	"github.com/spf13/cobra"
	cobraprompt "github.com/stromland/cobra-prompt"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
)

var command = &cobra.Command{
	Use:   "shell",
	Short: "Start a interactive shell in which you can execute any ptool commands.",
	Long: `Start a interactive shell in which you can execute any ptool commands.

It supports auto-completion for all commands and their arguments.

Keyboard Shortcuts
TAB    		Trigger auto-completion
Ctrl + A	Go to the beginning of the line (Home)
Ctrl + E	Go to the end of the line (End)
Ctrl + P	Previous command (Up arrow)
Ctrl + N	Next command (Down arrow)
Ctrl + F	Forward one character
Ctrl + B	Backward one character
Ctrl + D	Delete character under the cursor
Ctrl + H	Delete character before the cursor (Backspace)
Ctrl + W	Cut the word before the cursor to the clipboard
Ctrl + K	Cut the line after the cursor to the clipboard
Ctrl + U	Cut the line before the cursor to the clipboard
Ctrl + L	Clear the screen`,
	Args: cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
	RunE: shell,
}

const simpleHelp = `Type "<command> -h" to see full help of any command
Note client data will be cached in shell, run "purge [client]..." to purge cache
Use "exit" or Ctrl + D (in new line) to exit shell
To disable suggestions panel, add "shellMaxSuggestions = 0" line to the top of ptool.toml config file`

func init() {
	command.Long += fmt.Sprintf("\n\n%s\n\n%s", shellCommandsDescription, simpleHelp)
	cmd.RootCmd.AddCommand(command)
}

var ptoolPrompt = &cobraprompt.CobraPrompt{
	RootCmd:                  cmd.RootCmd,
	PersistFlagValues:        true,
	ShowHelpCommandAndFlags:  true,
	DisableCompletionCommand: true,
	GoPromptOptions: []prompt.Option{
		prompt.OptionTitle("ptool-shell"),
		prompt.OptionPrefix("> "),
		prompt.OptionShowCompletionAtStart(),
	},
	DynamicSuggestionsFunc: cmd.ShellDynamicSuggestionsFunc,
	OnErrorFunc: func(err error) {
		// error already printed in RootCmd
		// cmd.RootCmd.PrintErrln(err)
	},
	// InArgsParser: utils.ParseArgs,
	InArgsParser: func(in string) []string {
		words, err := shlex.Split(in)
		if err != nil {
			return nil
		}
		return words
	},
}

func shell(command *cobra.Command, args []string) error {
	if config.InShell {
		return fmt.Errorf(`you cann't run "shell" command in shell itself`)
	}
	if config.Fork || config.LockFile != "" {
		return fmt.Errorf("--fork or --lock flag can NOT be used with shell")
	}
	config.InShell = true
	ptoolPrompt.GoPromptOptions = append(ptoolPrompt.GoPromptOptions,
		prompt.OptionMaxSuggestion(uint16(config.Get().ShellMaxSuggestions)))
	history, err := cmd.ShellHistory.Load()
	if err == nil {
		ptoolPrompt.GoPromptOptions = append(ptoolPrompt.GoPromptOptions, prompt.OptionHistory(history))
	}
	cmd.RootCmd.AddCommand(shellCommands...)
	for name := range shellCommandSuggestions {
		cmd.AddShellCompletion(name, shellCommandSuggestions[name])
	}
	if !config.Get().Hushshell {
		fmt.Printf("Welcome to ptool shell\n")
		fmt.Printf("%s\n", shellCommandsDescription)
		fmt.Printf("%s\n", simpleHelp)
		fmt.Printf(`For full help of ptool shell, type "shell -h"` + "\n")
		fmt.Printf(`To mute this message, add "hushshell = true" line to the top of ptool.toml config file` + "\n")
	}
	ptoolPrompt.Run()
	return nil
}
