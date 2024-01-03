package versioncmd

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
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

var (
	impersonate string
)

func init() {
	command.Flags().StringVarP(&impersonate, "show-impersonate", "", "", "Show details of specified impersonate and exit")
	cmd.RootCmd.AddCommand(command)
}

func versioncmd(cmd *cobra.Command, args []string) error {
	if impersonate != "" {
		impersonateProfile := util.ImpersonateProfiles[impersonate]
		if impersonateProfile == nil {
			return fmt.Errorf("impersonate '%s' not supported", impersonate)
		}
		fmt.Printf("Impersonate '%s'\n", impersonate)
		fmt.Printf("- navigator: %s\n", impersonateProfile.Navigator)
		fmt.Printf("- comment: %s\n", impersonateProfile.Comment)
		fmt.Printf("- tls_ja3: %s\n", impersonateProfile.Ja3)
		fmt.Printf("- h2_fingerprint: %s\n", impersonateProfile.H2fingerpring)
		fmt.Printf("- http_request_headers:\n")
		for _, header := range impersonateProfile.Headers {
			value := header[1]
			if value == util.HTTP_HEADER_PLACEHOLDER {
				value = ""
			}
			fmt.Printf("  %s: %s\n", header[0], value)
		}
		return nil
	}
	fmt.Printf("ptool %s\n", version.Version)
	fmt.Printf("- os/type: %s\n", runtime.GOOS)
	fmt.Printf("- os/arch: %s\n", runtime.GOARCH)
	fmt.Printf("- go/version: %s\n", runtime.Version())
	fmt.Printf("- config_file: %s%c%s\n", config.ConfigDir, filepath.Separator, config.ConfigFile)
	fmt.Printf("- config_dir: %s\n", config.ConfigDir)
	fmt.Printf("- config/default_impersonate: %s\n", util.DEFAULT_IMPERSONATE)
	fmt.Printf("- config/supported_impersonates: %s, none\n", strings.Join(util.Impersonates, ", "))
	return nil
}
