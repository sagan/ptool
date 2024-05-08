package deletecategories

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:         "deletecategories {client} {category}...",
	Aliases:     []string{"delcats"},
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "deletecategories"},
	Short:       "Delete categories from client.",
	Long:        `Delete categories from client.`,
	Args:        cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE:        deletecategories,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func deletecategories(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	categories := args[1:]
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	err = clientInstance.DeleteCategories(categories)
	if err != nil {
		return err
	}
	return nil
}
