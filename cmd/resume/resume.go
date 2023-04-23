package resume

import (
	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/spf13/cobra"

	log "github.com/sirupsen/logrus"
)

var command = &cobra.Command{
	Use:   "resume <client> <infoHash>...",
	Short: "Resume torrents of client.",
	Long:  `Resume torrents of client.`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	Run:   resume,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func resume(cmd *cobra.Command, args []string) {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}
	infoHashes := args[1:]
	err = clientInstance.ResumeTorrents(infoHashes)
	if err != nil {
		log.Fatal(err)
	}
}
