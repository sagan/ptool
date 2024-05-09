package checktag

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/util"
)

var command = &cobra.Command{
	Use:         "checktag {client} {tag}",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "gettags"},
	Short:       "Check whether tag(s) exists in client.",
	Long: `Check whether tag(s) exists in client.
{tag}: comma-separated tag list.

If any one of {tag} list exists in client, exit with 0; Otherwise exit with a non-zero code.`,
	Args: cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
	RunE: checktag,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func checktag(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	tag := args[1]
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	tags := util.SplitCsv(tag)

	clientTags, err := clientInstance.GetTags()
	if err != nil {
		return fmt.Errorf("failed to get client %s tags: %w", clientName, err)
	}
	ok := false
	for _, tag := range tags {
		if slices.Contains(clientTags, tag) {
			fmt.Printf("✓ tag %s exists in client %s\n", tag, clientName)
			ok = true
		} else {
			fmt.Printf("✕ tag %s does NOT exist in client %s\n", tag, clientName)
		}
	}
	if !ok {
		return fmt.Errorf("none of tags %v exists in client %s", tags, clientName)
	}
	return nil
}
