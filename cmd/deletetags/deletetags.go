package deletetags

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:   "deletetags <client> <tags>...",
	Short: "Delete tags from client.",
	Long:  `Delete tags from client.`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	Run:   deletetags,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func deletetags(cmd *cobra.Command, args []string) {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}

	tags := args[1:]

	err = clientInstance.DeleteTags(tags...)
	clientInstance.Close()
	if err != nil {
		log.Fatalf("Failed to delete tags: %v", err)
	}
}
