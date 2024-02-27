package versioncmd

import (
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/util/impersonateutil"
	"github.com/sagan/ptool/version"
)

var command = &cobra.Command{
	Use:   "version",
	Short: "Display ptool version.",
	Long:  `Display ptool version.`,
	Args:  cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
	RunE:  versioncmd,
}

var (
	impersonate string
)

func init() {
	command.Flags().StringVarP(&impersonate, "show-impersonate", "", "", "Show details of specified impersonate and exit")
	cmd.RootCmd.AddCommand(command)
}

func versioncmd(cmd *cobra.Command, args []string) error {
	if impersonate != "" {
		impersonateProfile := impersonateutil.GetProfile(impersonate)
		if impersonateProfile == nil {
			return fmt.Errorf("impersonate '%s' not supported", impersonate)
		}
		impersonateProfile.Print()
		return nil
	}
	fmt.Printf("ptool %s\n", version.Version)
	fmt.Printf("- build/date: %s\n", version.Date)
	fmt.Printf("- build/commit: %s\n", version.Commit)
	fmt.Printf("- os/type: %s\n", runtime.GOOS)
	fmt.Printf("- os/arch: %s\n", runtime.GOARCH)
	fmt.Printf("- go/version: %s\n", runtime.Version())
	fmt.Printf("- config_file: %s%c%s\n", config.ConfigDir, filepath.Separator, config.ConfigFile)
	fmt.Printf("- config_dir: %s\n", config.ConfigDir)
	fmt.Printf("- config/default_impersonate: %s\n", impersonateutil.DEFAULT_IMPERSONATE)
	fmt.Printf("- config/supported_impersonates: %s, none\n", impersonateutil.GetAllProfileNames())
	return nil
}
