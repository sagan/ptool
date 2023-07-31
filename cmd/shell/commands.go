package shell

import (
	"fmt"
	"os"
	"os/exec"
	"sort"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
)

var (
	listMode      = false
	ShellCommands = []*cobra.Command{}
)

var Cd = &cobra.Command{
	Use:   "cd {dir}",
	Short: "(shell only) Change current working dir.",
	Long:  `Change current working dir.`,
	Args:  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		err := os.Chdir(args[0])
		if err != nil {
			return fmt.Errorf("failed to change work dir to %s: %v", args[0], err)
		}
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
			return fmt.Errorf("failed to get cwd: %v", err)
		}
		fmt.Printf("%s\n", cwd)
		return nil
	},
}

var execCmd = &cobra.Command{
	Use:     "! {external_program} [arg]...",
	Aliases: []string{"exec"},
	Short:   "(shell only) Execute external program.",
	Long:    `Execute external program.`,
	Args:    cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		externalCmd := exec.Command(args[0], args[1:]...)
		out, err := externalCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to execute %s: %v", args[0], err)
		}
		fmt.Printf("%s\n", out)
		return nil
	},
}

var lsCmd = &cobra.Command{
	Use:   "ls [-l] [dir]...",
	Short: "(shell only) List directory contents.",
	Long:  `List directory contents.`,
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
			sort.Slice(files, func(i, j int) bool {
				return files[i].Name() < files[j].Name()
			})
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
					fmt.Printf("%s", utils.QuoteFilename(file.Name()))
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
						fmt.Printf("%-1s  %10s  %19s  %-s\n", flag, "<error>", "<error>", file.Name())
					} else {
						fmt.Printf("%-1s  %10s  %19s  %-s\n",
							flag,
							utils.BytesSize(float64(info.Size())),
							utils.FormatTime(info.ModTime().Unix()),
							utils.QuoteFilename(file.Name()))
					}
				}
			}
		}
		return nil
	},
}

var exitCmd = &cobra.Command{
	Use:   "exit",
	Short: "(shell only) Exit shell",
	Run: func(command *cobra.Command, args []string) {
		cmd.Exit(0)
	},
}

var purgeCmd = &cobra.Command{
	Use:   "purge [client | site]...",
	Short: "(shell only) Purge client or site cache",
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
					log.Errorf("%s is not a client or site", name)
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

func init() {
	lsCmd.Flags().BoolVarP(&listMode, "list", "l", false, "Use a long listing format")
	ShellCommands = append(ShellCommands, pwdCwd, Cd, lsCmd, exitCmd, purgeCmd, execCmd)
}
