package deletetags

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:         "deletetags {client} {tags}...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "deletetags"},
	Short:       "Delete tags from client.",
	Long:        `Delete tags from client.`,
	Args:        cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE:        deletetags,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func deletetags(cmd *cobra.Command, args []string) error {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	tags := args[1:]

	err = clientInstance.DeleteTags(tags...)
	if err != nil {
		return fmt.Errorf("failed to delete tags: %v", err)
	}
	return nil
}
