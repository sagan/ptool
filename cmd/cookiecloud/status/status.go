package status

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd/cookiecloud"
	"github.com/sagan/ptool/config"
)

var (
	profile = ""
)

var command = &cobra.Command{
	Use:         "status",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "cookiecloud.status"},
	Short:       "Show cookiecloud servers status.",
	Long:        `Show cookiecloud servers status.`,
	RunE:        status,
}

func init() {
	command.Flags().StringVarP(&profile, "profile", "", "", "Comma-separated string, Set the used cookiecloud profile name(s). If not set, All cookiecloud profiles in config will be used")
	cookiecloud.Command.AddCommand(command)
}

func status(cmd *cobra.Command, args []string) error {
	cntError := int64(0)
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
	if len(cookiecloudProfiles) == 0 {
		return fmt.Errorf("no cookiecloud profile specified or found")
	}
	for _, profile := range cookiecloudProfiles {
		data, err := cookiecloud.GetCookiecloudData(profile.Server, profile.Uuid, profile.Password)
		if err != nil {
			fmt.Printf("✕cookiecloud server %s (uuid %s) test failed: %v\n", profile.Server, profile.Uuid, err)
			cntError++
		} else {
			fmt.Printf("✓cookiecloud server %s (uuid %s) test ok: %d site cookies found\n",
				profile.Server, profile.Uuid, len(data.Cookie_data))
		}
	}
	if cntError > 0 {
		return fmt.Errorf("%d errors", cntError)
	}
	return nil
}
