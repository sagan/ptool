package resume

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:     "resume <client> [<infoHash>...]",
	Aliases: []string{"start"},
	Short:   "Resume torrents of client.",
	Long: `Resume torrents of client.
<infoHash>...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done, _undone, _downloading, _seeding, _paused, _completed, _error.
`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run:  resume,
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

func resume(cmd *cobra.Command, args []string) {
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
		err = clientInstance.ResumeAllTorrents()
		if err != nil {
			clientInstance.Close()
			log.Fatal(err)
		}
	} else if len(infoHashes) > 0 {
		err = clientInstance.ResumeTorrents(infoHashes)
		if err != nil {
			clientInstance.Close()
			log.Fatal(err)
		}
	}
	clientInstance.Close()
}
