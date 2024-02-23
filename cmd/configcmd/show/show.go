package show

import (
	"fmt"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd/configcmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/util"
)

var command = &cobra.Command{
	Use:   "show {site | client | group | cookiecloud_profile | alias}...",
	Short: "Show effective config of config items.",
	Long: `Show effective config of config items.
It prints output in toml format.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: show,
}

func init() {
	configcmd.Command.AddCommand(command)
}

func show(cmd *cobra.Command, args []string) error {
	names := args

	for _, name := range names {
		if siteConfig := config.GetSiteConfig(name); siteConfig != nil {
			str, err := toml.Marshal(util.StructToMap(*siteConfig, true, true))
			if err != nil {
				fmt.Printf("# %s : failed to get detailed configuration: %v\n", name, err)
				continue
			}
			fmt.Printf("# %s\n[[sites]]\n%s\n", name, str)
		} else if clientConfig := config.GetClientConfig(name); clientConfig != nil {
			str, err := toml.Marshal(util.StructToMap(*clientConfig, true, true))
			if err != nil {
				fmt.Printf("# %s : failed to get detailed configuration: %v\n", name, err)
				continue
			}
			fmt.Printf("# %s\n[[clients]]\n%s\n", name, str)
		} else if groupConfig := config.GetGroupConfig(name); groupConfig != nil {
			str, err := toml.Marshal(util.StructToMap(*groupConfig, true, true))
			if err != nil {
				fmt.Printf("# %s : failed to get detailed configuration: %v\n", name, err)
				continue
			}
			fmt.Printf("# %s\n[[groups]]\n%s\n", name, str)
		} else if cookiecloudConfig := config.GetCookiecloudConfig(name); cookiecloudConfig != nil {
			str, err := toml.Marshal(util.StructToMap(*cookiecloudConfig, true, true))
			if err != nil {
				fmt.Printf("# %s : failed to get detailed configuration: %v\n", name, err)
				continue
			}
			fmt.Printf("# %s\n[[cookieclouds]]\n%s\n", name, str)
		} else if aliasConfig := config.GetAliasConfig(name); aliasConfig != nil {
			str, err := toml.Marshal(util.StructToMap(*aliasConfig, true, true))
			if err != nil {
				fmt.Printf("# %s : failed to get detailed configuration: %v\n", name, err)
				continue
			}
			fmt.Printf("# %s\n[[aliases]]\n%s\n", name, str)
		} else {
			fmt.Printf("# '%s' does NOT match with any config item\n\n", name)
		}
	}
	return nil
}
