package setcategory

import (
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:   "setcategory <client> <category> <infoHashes>...",
	Short: "Set category of torrents in client.",
	Long: `Set category of torrents in client.
infoHashes...: infoHash list of torrents. It's possible to use some special values to target multiple torrents:
_all, _completed (or _done), _error`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(3), cobra.OnlyValidArgs),
	Run:  createtags,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func createtags(cmd *cobra.Command, args []string) {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}

	cat := args[1]

	args = args[2:]
	infoHashes := []string{}
	for _, arg := range args {
		if strings.HasPrefix(arg, "_") {
			if arg == "_all" {
				err = clientInstance.SetAllTorrentsCatetory(cat)
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

	err = clientInstance.SetTorrentsCatetory(infoHashes, cat)
	if err != nil {
		log.Fatalf("Failed to set torrents category: %v", err)
	}
}
