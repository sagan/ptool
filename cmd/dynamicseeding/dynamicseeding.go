package dynamicseeding

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/site"
)

var command = &cobra.Command{
	Use:         "dynamicseeding {client} {site}",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "dynamicseeding"},
	Short:       "Dynamic seeding torrents of sites.",
	Long:        `Dynamic seeding torrents of sites.`,
	Args:        cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
	RunE:        dynamicseeding,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func dynamicseeding(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	sitename := args[1]
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}
	siteInstance, err := site.CreateSite(sitename)
	if err != nil {
		return fmt.Errorf("failed to create site: %v", err)
	}
	result, err := doDynamicSeeding(clientInstance, siteInstance)
	if err != nil {
		return err
	}
	result.Print(os.Stdout)
	return nil
}
