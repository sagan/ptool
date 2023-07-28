package gettags

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:   "gettags <client>",
	Short: "Get all tags of client.",
	Long:  `Get all tags of client.`,
	Args:  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE:  gettags,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func gettags(cmd *cobra.Command, args []string) error {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	tags, err := clientInstance.GetTags()
	clientInstance.Close()
	if err != nil {
		return fmt.Errorf("failed to get tags: %v", err)
	}
	fmt.Printf("%s\n", strings.Join(tags, ", "))
	return nil
}
