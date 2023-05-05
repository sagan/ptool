package resume

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:   "resume <client> [<infoHash>...]",
	Short: "Resume torrents of client",
	Long: `Resume torrents of client
<infoHash>...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done,  _downloading, _seeding, _paused, _completed, _error
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
	command.Flags().StringVarP(&filter, "filter", "f", "", "filter torrents by name")
	command.Flags().StringVarP(&category, "category", "c", "", "filter torrents by category")
	command.Flags().StringVarP(&tag, "tag", "t", "", "filter torrents by tag")
	cmd.RootCmd.AddCommand(command)
}

func resume(cmd *cobra.Command, args []string) {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}
	args = args[1:]
	if category == "" && tag == "" && filter == "" && len(args) == 0 {
		log.Fatalf("You must provide at least a condition flag or hashFilter")
	}
	infoHashes, err := client.SelectTorrents(clientInstance, category, tag, filter, args...)
	if err != nil {
		log.Fatal(err)
	}
	if infoHashes == nil {
		err = clientInstance.ResumeAllTorrents()
		if err != nil {
			log.Fatal(err)
		}
	} else if len(infoHashes) > 0 {
		err = clientInstance.ResumeTorrents(infoHashes)
		if err != nil {
			log.Fatal(err)
		}
	}
}
