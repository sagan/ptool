package versioncmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/util"
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
	showJson    = false
	impersonate string
)

func init() {
	command.Flags().BoolVarP(&showJson, "json", "", false, "Show output in json format")
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
	configFile := filepath.Join(config.ConfigDir, config.ConfigFile)
	if showJson {
		versionStr, versionLabel, _ := strings.Cut(strings.TrimPrefix(version.Version, "v"), "-")
		versionNumbers := strings.Split(versionStr, ".")
		data := map[string]any{
			"Version":               version.Version,
			"VersionMajor":          util.ParseInt(versionNumbers[0]),
			"VersionMinor":          util.ParseInt(versionNumbers[1]),
			"VersionPatch":          util.ParseInt(versionNumbers[2]),
			"VersionLabel":          versionLabel,
			"Date":                  version.Date,
			"Commit":                version.Commit,
			"OsType":                runtime.GOOS,
			"OsArch":                runtime.GOARCH,
			"GoVersion":             runtime.Version(),
			"ConfigFile":            configFile,
			"ConfigDir":             config.ConfigDir,
			"DefaultImpersonate":    impersonateutil.DEFAULT_IMPERSONATE,
			"SupportedImpersonates": impersonateutil.GetAllProfileNames(),
		}
		util.PrintJson(os.Stdout, data)
		return nil
	}
	fmt.Printf("ptool %s\n", version.Version)
	fmt.Printf("- build/date: %s\n", version.Date)
	fmt.Printf("- build/commit: %s\n", version.Commit)
	fmt.Printf("- os/type: %s\n", runtime.GOOS)
	fmt.Printf("- os/arch: %s\n", runtime.GOARCH)
	fmt.Printf("- go/version: %s\n", runtime.Version())
	fmt.Printf("- config_file: %s\n", configFile)
	fmt.Printf("- config_dir: %s\n", config.ConfigDir)
	fmt.Printf("- config/default_impersonate: %s\n", impersonateutil.DEFAULT_IMPERSONATE)
	fmt.Printf("- config/supported_impersonates: %s, none\n", impersonateutil.GetAllProfileNames())
	return nil
}
