package pause

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:     "pause <client> [<infoHash>...]",
	Aliases: []string{"stop"},
	Short:   "Pause torrents of client.",
	Long: `Pause torrents of client.
<infoHash>...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done,  _downloading, _seeding, _paused, _completed, _error.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run:  pause,
}

var (
	category = ""
	tag      = ""
	filter   = ""
)

func init() {
	command.Flags().StringVarP(&filter, "filter", "f", "", "Filter torrents by name")
	command.Flags().StringVarP(&category, "category", "c", "", "Filter torrents by category")
	command.Flags().StringVarP(&tag, "tag", "t", "", "Filter torrents by tag")
	cmd.RootCmd.AddCommand(command)
}

func pause(cmd *cobra.Command, args []string) {
	clientName := args[0]
	args = args[1:]
	if category == "" && tag == "" && filter == "" && len(args) == 0 {
		log.Fatalf("You must provide at least a condition flag or hashFilter")
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		log.Fatal(err)
	}

	infoHashes, err := client.SelectTorrents(clientInstance, category, tag, filter, args...)
	if err != nil {
		clientInstance.Close()
		log.Fatal(err)
	}
	if infoHashes == nil {
		err = clientInstance.PauseAllTorrents()
		if err != nil {
			clientInstance.Close()
			log.Fatal(err)
		}
	} else if len(infoHashes) > 0 {
		err = clientInstance.PauseTorrents(infoHashes)
		if err != nil {
			clientInstance.Close()
			log.Fatal(err)
		}
	}
	clientInstance.Close()
}
