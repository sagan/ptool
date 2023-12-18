package cookiecloud

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
)

var Command = &cobra.Command{
	Use:     "cookiecloud",
	Aliases: []string{"cc"},
	Short:   "Use cookiecloud to sync site cookies or import sites.",
	Long: `Use cookiecloud to sync site cookies or import sites.
To use this feature, add the cookiecloud servers to config file, e.g. :

ptool.toml
----------
[[cookieclouds]]
server = 'https://cookiecloud.example.com'
uuid = 'uuid'
password = 'password'
----------

See also:
* CookieCloud: https://github.com/easychen/CookieCloud`,
	Args: cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
}

func init() {
	cmd.RootCmd.AddCommand(Command)
}

func ParseProfile(profile string) []*config.CookiecloudConfigStruct {
	cookiecloudProfiles := []*config.CookiecloudConfigStruct{}
	if profile == "" {
		for _, profile := range config.Get().Cookieclouds {
			if profile.Disabled {
				continue
			}
			cookiecloudProfiles = append(cookiecloudProfiles, profile)
		}
	} else {
		names := strings.Split(profile, ",")
		for _, name := range names {
			profile := config.GetCookiecloudConfig(name)
			if profile != nil {
				cookiecloudProfiles = append(cookiecloudProfiles, profile)
			}
		}
	}
	return cookiecloudProfiles
}
