package versioncmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/version"
)

var command = &cobra.Command{
	Use:   "version",
	Short: "Display ptool version",
	Long:  `Display ptool version`,
	Args:  cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
	Run:   versioncmd,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func versioncmd(cmd *cobra.Command, args []string) {
	fmt.Printf("ptool %s\n", version.Version)
	fmt.Printf("- os/type: %s\n", runtime.GOOS)
	fmt.Printf("- os/arch: %s\n", runtime.GOARCH)
	fmt.Printf("- go/version: %s\n", runtime.Version())
}
