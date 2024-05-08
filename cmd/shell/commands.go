package shell

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/c-bata/go-prompt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/shell/suggest"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
)

var (
	force    = false
	listMode = false
	clear    = false
)

var cdCmd = &cobra.Command{
	Use:         "cd {dir}",
	Short:       "(shell only) Change current working dir.",
	Long:        `Change current working dir.`,
	Args:        cobra.MatchAll(cobra.MaximumNArgs(1), cobra.OnlyValidArgs),
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "cd"},
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := ""
		var err error
		if len(args) == 0 {
			dir, err = os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get user home dir: %w", err)
			}
		} else {
			dir = args[0]
		}
		err = os.Chdir(dir)
		if err != nil {
			return fmt.Errorf("failed to change work dir to %s: %w", args[0], err)
		}
		pwd, _ := os.Getwd()
		fmt.Printf("Changed work dir to %s\n", pwd)
		return nil
	},
}

var pwdCwd = &cobra.Command{
	Use:   "pwd",
	Short: "(shell only) Print current working dir.",
	Long:  `Print current working dir.`,
	Args:  cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get cwd: %w", err)
		}
		fmt.Printf("%s\n", cwd)
		return nil
	},
}

var execCmd = &cobra.Command{
	Use:                "! {external_program} [arg]...",
	Aliases:            []string{"exec"},
	Short:              `(shell only) (alias "exec") Execute external program.`,
	Long:               `Execute external program.`,
	DisableFlagParsing: true,
	Args:               cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		externalCmd := exec.Command(args[0], args[1:]...)
		out, err := externalCmd.CombinedOutput()
		fmt.Printf("%s\n", out)
		if err != nil {
			return fmt.Errorf("failed to execute %s: %w", args[0], err)
		}
		return nil
	},
}

var lsCmd = &cobra.Command{
	Use:         "ls [-l] [dir]...",
	Aliases:     []string{"dir"},
	Short:       "(shell only) List directory contents.",
	Long:        `List directory contents.`,
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		dirs := []string{}
		dirs = append(dirs, args...)
		if len(dirs) == 0 {
			dirs = append(dirs, ".")
		}
		for i, dir := range dirs {
			files, err := os.ReadDir(dir)
			if err != nil {
				log.Errorf("ls: cannot access '%s': %v", dir, err)
				continue
			}
			if i > 0 {
				fmt.Printf("\n")
			}
			if len(dirs) > 1 {
				fmt.Printf("%s:\n", dir)
			}
			if !listMode {
				for i, file := range files {
					if i > 0 {
						fmt.Printf("  ")
					}
					fmt.Printf("%s", util.QuoteFilename(file.Name()))
				}
				fmt.Printf("\n")
			} else {
				for _, file := range files {
					flag := ""
					if file.IsDir() {
						flag = "d"
					} else {
						flag = "-"
					}
					info, err := file.Info()
					if err != nil {
						fmt.Printf("%-1s  %6s  %19s  %-s\n", flag, "!error", "<error>", file.Name())
					} else {
						fmt.Printf("%-1s  %6s  %19s  %-s\n",
							flag,
							util.BytesSizeAround(float64(info.Size())),
							util.FormatTime(info.ModTime().Unix()),
							util.QuoteFilename(file.Name()))
					}
				}
			}
		}
		return nil
	},
}

var exitCmd = &cobra.Command{
	Use:     "exit",
	Aliases: []string{"quit"},
	Short:   "(shell only) Exit shell",
	Run: func(command *cobra.Command, args []string) {
		if force {
			os.Exit(0)
		}
		cmd.Exit(0)
	},
}

var exitfCmd = &cobra.Command{
	Use:   "exitf",
	Short: `(shell only) Alias of "exit -f". Exit shell immediately & forcely`,
	Args:  cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		os.Exit(0)
		return nil
	},
}

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "(shell only) List history of executed commands in shell",
	Run: func(command *cobra.Command, args []string) {
		if clear {
			cmd.ShellHistory.Clear()
			return
		}
		history, _ := cmd.ShellHistory.Load()
		for i, h := range history {
			fmt.Printf("%-5d  %s\n", i, h)
		}
	},
}

var purgeCmd = &cobra.Command{
	Use:         "purge [client | site]...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "purge"},
	Short:       "(shell only) Purge client or site cache",
	Long: `(shell only) Purge client or site cache
If no args provided, the cache of ALL clients and sites will be purged`,
	RunE: func(command *cobra.Command, args []string) error {
		errorCnt := int64(0)
		if len(args) == 0 {
			client.Purge("")
			site.Purge("")
		} else {
			for _, name := range args {
				if client.ClientExists(name) {
					client.Purge(name)
				} else if site.SiteExists(name) {
					site.Purge(name)
				} else {
					log.Errorf("%s is not a client nor site", name)
					errorCnt++
				}
			}
		}
		if errorCnt > 0 {
			return fmt.Errorf("%d errors", errorCnt)
		}
		return nil
	},
}

func cdCmdSuggestion(document *prompt.Document) []prompt.Suggest {
	info := suggest.Parse(document)
	if info.LastArgIndex < 1 {
		return nil
	}
	if info.LastArgIsFlag {
		return nil
	}
	return suggest.DirArg(info.MatchingPrefix)
}

func lsCmdSuggestion(document *prompt.Document) []prompt.Suggest {
	info := suggest.Parse(document)
	if info.LastArgIndex < 1 {
		return nil
	}
	if info.LastArgIsFlag {
		return nil
	}
	return suggest.FileArg(info.MatchingPrefix, "", false)
}

func purgeCmdSuggestion(document *prompt.Document) []prompt.Suggest {
	info := suggest.Parse(document)
	if info.LastArgIndex < 1 {
		return nil
	}
	if info.LastArgIsFlag {
		return nil
	}
	return suggest.ClientArg(info.MatchingPrefix)
}

var shellCommands = []*cobra.Command{pwdCwd, cdCmd, lsCmd, historyCmd, exitCmd, exitfCmd, purgeCmd, execCmd}

var shellCommandSuggestions = map[string](func(document *prompt.Document) []prompt.Suggest){
	"cd":    cdCmdSuggestion,
	"ls":    lsCmdSuggestion,
	"purge": purgeCmdSuggestion,
}

var shellCommandsDescription = "In addition to normal ptool commands, you can use the shell commands here:\n"

func init() {
	exitCmd.Flags().BoolVarP(&force, "force", "f", false, "Force exit immediately. Do NOT clean resources")
	lsCmd.Flags().BoolVarP(&listMode, "list", "l", false, "Use a long listing format")
	historyCmd.Flags().BoolVarP(&clear, "clear", "c", false, "Clear history")
	for i, shellCmd := range shellCommands {
		if i > 0 {
			shellCommandsDescription += "\n"
		}
		shellCommandsDescription += fmt.Sprintf("* %s : %s", shellCmd.Name(), shellCmd.Short)
	}
}
