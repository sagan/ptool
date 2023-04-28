package reannounce

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:   "reannounce <client> <infoHash>...",
	Short: "Reannounce torrents of client.",
	Long: `Reannounce torrents of client.
infoHashes...: infoHash list of torrents. It's possible to use some special values to target multiple torrents:
_all, _completed (or _done), _error`,
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
	infoHashes := []string{}
	for _, arg := range args {
		if strings.HasPrefix(arg, "_") {
			if arg == "_all" {
				err = clientInstance.ReannounceAllTorrents()
				if err != nil {
					log.Fatal(err)
				}
				os.Exit(0)
			}
			hashes, err := client.GetClientTorrentInfoHashes(clientInstance, arg, "")
			if err != nil {
				log.Errorf("Failed to fetch %s state torrents from client", arg)
			} else {
				infoHashes = append(infoHashes, hashes...)
			}
		}
	}

	err = clientInstance.ReannounceTorrents(infoHashes)
	if err != nil {
		log.Fatal(err)
	}
}
