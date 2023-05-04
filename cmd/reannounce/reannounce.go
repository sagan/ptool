package reannounce

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:   "reannounce <client> <infoHash>...",
	Short: "Reannounce torrents of client",
	Long: `Reannounce torrents of client
<infoHash>...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done,  _downloading, _seeding, _paused, _completed, _error`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	Run:  reannounce,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func reannounce(cmd *cobra.Command, args []string) {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}
	args = args[1:]
	infoHashes, err := client.SelectTorrents(clientInstance, "", "", "", args...)
	if err != nil {
		log.Fatal(err)
	}
	if infoHashes == nil {
		err = clientInstance.ReannounceAllTorrents()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		err = clientInstance.ReannounceTorrents(infoHashes)
		if err != nil {
			log.Fatal(err)
		}
	}
}
