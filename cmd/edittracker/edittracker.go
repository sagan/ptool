package edittracker

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/utils"
)

var command = &cobra.Command{
	Use:   "edittracker <client> [<infoHash>...]",
	Short: "Edit tracker of torrents in client.",
	Long: `Edit tracker of torrents in client, replace the old tracker url with the new one.
<infoHash>...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done,  _downloading, _seeding, _paused, _completed, _error.

A torrent will not be updated if old tracker does NOT exist in it's trackers list.
It may return an error in such case or not, depending on specific client implementation.

Examples:
ptool edittracker <client> _all --old-tracker "https://..." --new-tracker "https://..."
ptool edittracker <client> _all --old-tracker old-tracker.com --new-tracker new-tracker.com --replace-host
`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run:  edittracker,
}

var (
	dryRun      = false
	replaceHost = false
	category    = ""
	tag         = ""
	filter      = ""
	oldTracker  = ""
	newTracker  = ""
)

func init() {
	command.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Dry run. Do NOT actually modify torrent trackers")
	command.Flags().BoolVarP(&replaceHost, "replace-host", "", false, "Replace host mode. If set, --old-tracker should be the old host (hostname[:port]) instead of full url, the --new-tracker can either be a host or full url")
	command.Flags().StringVarP(&filter, "filter", "f", "", "Filter torrents by name")
	command.Flags().StringVarP(&category, "category", "c", "", "Filter torrents by category")
	command.Flags().StringVarP(&tag, "tag", "t", "", "Filter torrents by tag")
	command.Flags().StringVarP(&oldTracker, "old-tracker", "", "", "Set the old tracker")
	command.Flags().StringVarP(&newTracker, "new-tracker", "", "", "Set the new tracker")
	command.MarkFlagRequired("old-tracker")
	command.MarkFlagRequired("new-tracker")
	cmd.RootCmd.AddCommand(command)
}

func edittracker(cmd *cobra.Command, args []string) {
	clientName := args[0]
	args = args[1:]
	if !replaceHost && (!utils.IsUrl(oldTracker) || !utils.IsUrl(newTracker)) {
		log.Fatalf("Both --old-tracker and --new-tracker MUST be valid URL ( 'http(s)://...' )")
	}
	if category == "" && tag == "" && filter == "" && len(args) == 0 {
		log.Fatalf("You must provide at least a condition flag or hashFilter")
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		log.Fatal(err)
	}

	torrents, err := client.QueryTorrents(clientInstance, category, tag, filter, args...)
	if err != nil {
		clientInstance.Close()
		log.Fatal(err)
	}
	if len(torrents) == 0 {
		clientInstance.Close()
		log.Infof("No matched torrents found")
		os.Exit(0)
	}
	if !dryRun {
		log.Warnf("Found %d torrents, will edit their trackers (%s => %s, replaceHost=%t) in few seconds. Press Ctrl+C to stop",
			len(torrents), oldTracker, newTracker, replaceHost)
	}
	utils.Sleep(3)
	cntError := int64(0)
	for _, torrent := range torrents {
		fmt.Printf("Edit torrent %s (%s) tracker\n", torrent.InfoHash, torrent.Name)
		if dryRun {
			continue
		}
		err := clientInstance.EditTorrentTracker(torrent.InfoHash, oldTracker, newTracker, replaceHost)
		if err != nil {
			log.Errorf("Failed to edit tracker: %v\n", err)
			cntError++
		}
	}
	clientInstance.Close()
	if cntError > 0 {
		os.Exit(1)
	}
}
