package version

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
)

var command = &cobra.Command{
	Use:   "version",
	Short: "Display ptool version",
	Long:  `Display ptool version`,
	Args:  cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
	Run:   version,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func version(cmd *cobra.Command, args []string) {
	fmt.Printf("ptool version v-%s\n", config.VERSION)
	fmt.Printf("- os/type: %s\n", runtime.GOOS)
	fmt.Printf("- os/arch: %s\n", runtime.GOARCH)
	fmt.Printf("- go/version: %s\n", runtime.Version())
}
