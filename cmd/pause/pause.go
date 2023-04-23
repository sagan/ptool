package pause

import (
	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/spf13/cobra"

	log "github.com/sirupsen/logrus"
)

var command = &cobra.Command{
	Use:   "pause <client> <infoHash>...",
	Short: "Pause torrents of client.",
	Long:  `Pause torrents of client.`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	Run:   pause,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func pause(cmd *cobra.Command, args []string) {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}
	infoHashes := args[1:]
	err = clientInstance.PauseTorrents(infoHashes)
	if err != nil {
		log.Fatal(err)
	}
}
