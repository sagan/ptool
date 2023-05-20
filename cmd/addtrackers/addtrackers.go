package addtrackers

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
	Use:   "addtrackers <client> [<infoHash>...]",
	Short: "Add new trackers to torrents of client",
	Long: `Add new trackers to torrents of client,
<infoHash>...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done,  _downloading, _seeding, _paused, _completed, _error

Example:
ptool addtrackers <client> --tracker "https://..."
--tracker flag can be used many times.
`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run:  addtrackers,
}

var (
	dryRun   = false
	category = ""
	tag      = ""
	filter   = ""
	trackers = []string{}
)

func init() {
	command.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Dry run. Do NOT actually modify torrent trackers")
	command.Flags().StringVarP(&filter, "filter", "f", "", "Filter torrents by name")
	command.Flags().StringVarP(&category, "category", "c", "", "Filter torrents by category")
	command.Flags().StringVarP(&tag, "tag", "t", "", "Filter torrents by tag")
	command.Flags().StringArrayVarP(&trackers, "tracker", "", nil, "Set the tracker to add. Can be used multiple times")
	command.MarkFlagRequired("tracker")
	cmd.RootCmd.AddCommand(command)
}

func addtrackers(cmd *cobra.Command, args []string) {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}
	args = args[1:]
	if category == "" && tag == "" && filter == "" && len(args) == 0 {
		log.Fatalf("You must provide at least a condition flag or hashFilter")
	}
	if len(trackers) == 0 {
		log.Fatalf("At least an --tracker MUST be provided")
	}
	for _, tracker := range trackers {
		if !utils.IsUrl(tracker) {
			log.Fatalf("The provided tracker %s is not a valid URL", tracker)
		}
	}
	torrents, err := client.QueryTorrents(clientInstance, category, tag, filter, args...)
	if err != nil {
		log.Fatal(err)
	}
	if len(torrents) == 0 {
		log.Infof("No matched torrents found")
		os.Exit(0)
	}
	if !dryRun {
		log.Warnf("Found %d torrents, will add %d trackers to them in 3 seconds. Press Ctrl+C to stop",
			len(torrents), len(trackers))
	}
	utils.Sleep(3)
	cntError := int64(0)
	for _, torrent := range torrents {
		fmt.Printf("Add trackers to torrent %s (%s)\n", torrent.InfoHash, torrent.Name)
		if dryRun {
			continue
		}
		err := clientInstance.AddTorrentTrackers(torrent.InfoHash, trackers)
		if err != nil {
			log.Errorf("Failed to add trackers: %v\n", err)
			cntError++
		}
	}
	if cntError > 0 {
		os.Exit(1)
	}
}
