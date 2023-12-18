package versioncmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/version"
)

var command = &cobra.Command{
	Use:   "version",
	Short: "Display ptool version.",
	Long:  `Display ptool version.`,
	Args:  cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
	RunE:  versioncmd,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func versioncmd(cmd *cobra.Command, args []string) error {
	fmt.Printf("ptool %s\n", version.Version)
	fmt.Printf("- os/type: %s\n", runtime.GOOS)
	fmt.Printf("- os/arch: %s\n", runtime.GOARCH)
	fmt.Printf("- go/version: %s\n", runtime.Version())
	fmt.Printf("- config/default_http_request_headers:\n")
	for _, header := range util.CHROME_HTTP_REQUEST_HEADERS {
		value := header[1]
		if value == util.HTTP_HEADER_PLACEHOLDER {
			value = ""
		}
		fmt.Printf("  %s: %s\n", header[0], value)
	}
	return nil
}
