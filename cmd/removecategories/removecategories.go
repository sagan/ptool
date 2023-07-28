package removecategories

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:   "removecategories <client> <categories>...",
	Short: "Remove categories from client.",
	Long:  `Remove categories from client.`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE:  removecategories,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func removecategories(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	categories := args[1:]
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	err = clientInstance.RemoveCategories(categories)
	clientInstance.Close()
	if err != nil {
		return err
	}
	return nil
}
