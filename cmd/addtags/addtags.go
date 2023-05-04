package addtags

import (
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
)

var command = &cobra.Command{
	Use:   "addtags <client> <tags> <infoHash>...",
	Short: "Add tags to torrents in client",
	Long: `Add tags to torrents in client
<tags> : comma-seperated tags list
<infoHash>...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done,  _downloading, _seeding, _paused, _completed, _error`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(3), cobra.OnlyValidArgs),
	Run:  addtags,
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

func addtags(cmd *cobra.Command, args []string) {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}
	tags := strings.Split(args[1], ",")
	args = args[2:]
	infoHashes, err := client.SelectTorrents(clientInstance, category, tag, filter, args...)
	if err != nil {
		log.Fatal(err)
	}
	if infoHashes == nil {
		err = clientInstance.AddTagsToAllTorrents(tags)
		if err != nil {
			log.Fatal(err)
		}
	} else if len(infoHashes) > 0 {
		err = clientInstance.AddTagsToTorrents(infoHashes, tags)
		if err != nil {
			log.Fatal(err)
		}
	}
}
