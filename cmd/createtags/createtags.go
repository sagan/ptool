package createtags

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:         "createtags {client} {tags}...",
	Aliases:     []string{"createtag"},
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "createtags"},
	Short:       "Create tags in client.",
	Long:        `Create tags in client.`,
	Args:        cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE:        createtags,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func createtags(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	tags := args[1:]
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	err = clientInstance.CreateTags(tags...)
	if err != nil {
		return fmt.Errorf("Failed to create tags: %w", err)
	}
	return nil
}
