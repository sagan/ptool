package status

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd/cookiecloud"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/util"
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
	command.Flags().StringVarP(&profile, "profile", "", "",
		"Comma-separated list. Set the used cookiecloud profile name(s). "+
			"If not set, All cookiecloud profiles in config will be used")
	cookiecloud.Command.AddCommand(command)
}

func status(cmd *cobra.Command, args []string) error {
	errorCnt := int64(0)
	cookiecloudProfiles := cookiecloud.ParseProfile(profile)
	if len(cookiecloudProfiles) == 0 {
		return fmt.Errorf("no cookiecloud profile specified or found")
	}
	for _, profile := range cookiecloudProfiles {
		data, err := cookiecloud.GetCookiecloudData(profile.Server, profile.Uuid, profile.Password,
			config.GetProxy(profile.Proxy), profile.Timeout)
		if err != nil {
			fmt.Printf("✕cookiecloud server %s (uuid %s) test failed: %v\n",
				util.ParseUrlHostname(profile.Server), profile.Uuid, err)
			errorCnt++
		} else {
			fmt.Printf("✓cookiecloud server %s (uuid %s) test ok: cookies of %d domains found\n",
				util.ParseUrlHostname(profile.Server), profile.Uuid, len(data.Cookie_data))
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
