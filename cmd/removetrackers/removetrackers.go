package removetrackers

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/utils"
)

var command = &cobra.Command{
	Use:         "removetrackers {client} [-c category] [-t tags] [-f filter] [infoHash]...",
	Annotations: map[string](string){"cobra-prompt-dynamic-suggestions": "removetrackers"},
	Short:       "Remove trackers from torrents of client.",
	Long: `Remove trackers from torrents of client.
[infoHash]...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done, _undone, _downloading, _seeding, _paused, _completed, _error.

Example:
ptool removetrackers <client> <infoHashes...> --tracker "https://..."
--tracker flag can be used many times.
`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: removetrackers,
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
	command.Flags().StringVarP(&filter, "filter", "", "", "Filter torrents by name")
	command.Flags().StringVarP(&category, "category", "", "", "Filter torrents by category")
	command.Flags().StringVarP(&tag, "tag", "", "", "Filter torrents by tag. Comma-separated string list. Torrent which tags contain any one in the list will match")
	command.Flags().StringArrayVarP(&trackers, "tracker", "", nil, "Set the tracker to remove. Can be used multiple times")
	command.MarkFlagRequired("tracker")
	cmd.RootCmd.AddCommand(command)
}

func removetrackers(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	args = args[1:]
	if category == "" && tag == "" && filter == "" && len(args) == 0 {
		return fmt.Errorf("you must provide at least a condition flag or hashFilter")
	}
	if len(trackers) == 0 {
		return fmt.Errorf("at least an --tracker MUST be provided")
	}
	for _, tracker := range trackers {
		if !utils.IsUrl(tracker) {
			return fmt.Errorf("the provided tracker %s is not a valid URL", tracker)
		}
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	torrents, err := client.QueryTorrents(clientInstance, category, tag, filter, args...)
	if err != nil {
		return err
	}
	if len(torrents) == 0 {
		log.Infof("No matched torrents found")
		return nil
	}
	if !dryRun {
		log.Warnf("Found %d torrents, will remove %d trackers to them in 3 seconds. Press Ctrl+C to stop",
			len(torrents), len(trackers))
	}
	utils.Sleep(3)
	errorCnt := int64(0)
	for _, torrent := range torrents {
		fmt.Printf("Remove trackers from torrent %s (%s)\n", torrent.InfoHash, torrent.Name)
		if dryRun {
			continue
		}
		err := clientInstance.RemoveTorrentTrackers(torrent.InfoHash, trackers)
		if err != nil {
			log.Errorf("Failed to remove trackers: %v\n", err)
			errorCnt++
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
