package resume

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:   "resume <client> <infoHash>...",
	Short: "Resume torrents of client",
	Long: `Resume torrents of client
<infoHash>...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done,  _downloading, _seeding, _paused, _completed, _error
`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	Run:  resume,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func resume(cmd *cobra.Command, args []string) {
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
		err = clientInstance.ResumeAllTorrents()
		if err != nil {
			log.Fatal(err)
		}
	} else {
		err = clientInstance.ResumeTorrents(infoHashes)
		if err != nil {
			log.Fatal(err)
		}
	}
}
