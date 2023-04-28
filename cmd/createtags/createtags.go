package createtags

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:   "createtags <client> <tags>...",
	Short: "Create tags in client.",
	Long:  `Create tags in client.`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	Run:   createtags,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func createtags(cmd *cobra.Command, args []string) {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}

	tags := args[1:]

	err = clientInstance.CreateTags(tags...)
	if err != nil {
		log.Fatalf("Failed to create tags: %v", err)
	}
}
