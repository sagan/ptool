package edittracker

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/util"
)

var command = &cobra.Command{
	Use:         "edittracker <client> [--category category] [--tag tag] [--filter filter] [infoHash]... --old-tracker {url} --new-tracker {url} [--replace-host]",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "edittracker"},
	Short:       "Edit tracker of torrents in client.",
	Long: `Edit tracker of torrents in client, replace the old tracker url with the new one.
[infoHash]...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done, _undone, _downloading, _seeding, _paused, _completed, _error.

A torrent will not be updated if old tracker does NOT exist in it's trackers list.
It may return an error in such case or not, depending on specific client implementation.

Examples:
ptool edittracker <client> _all --old-tracker "https://..." --new-tracker "https://..."
ptool edittracker <client> _all --old-tracker old-tracker.com --new-tracker new-tracker.com --replace-host
ptool edittracker <client> _all --old-tracker old-tracker.com --new-tracker "https://..." --replace-host
`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: edittracker,
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
	command.Flags().StringVarP(&filter, "filter", "", "", "Filter torrents by name")
	command.Flags().StringVarP(&category, "category", "", "", "Filter torrents by category")
	command.Flags().StringVarP(&tag, "tag", "", "", "Filter torrents by tag. Comma-separated string list. Torrent which tags contain any one in the list will match")
	command.Flags().StringVarP(&oldTracker, "old-tracker", "", "", "Set the old tracker")
	command.Flags().StringVarP(&newTracker, "new-tracker", "", "", "Set the new tracker")
	command.MarkFlagRequired("old-tracker")
	command.MarkFlagRequired("new-tracker")
	cmd.RootCmd.AddCommand(command)
}

func edittracker(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	args = args[1:]
	if !replaceHost && (!util.IsUrl(oldTracker) || !util.IsUrl(newTracker)) {
		return fmt.Errorf("both --old-tracker and --new-tracker MUST be valid URL ( 'http(s)://...' )")
	}
	if category == "" && tag == "" && filter == "" && len(args) == 0 {
		return fmt.Errorf("you must provide at least a condition flag or hashFilter")
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
		log.Warnf("Found %d torrents, will edit their trackers (%s => %s, replaceHost=%t) in few seconds. Press Ctrl+C to stop",
			len(torrents), oldTracker, newTracker, replaceHost)
	}
	util.Sleep(3)
	errorCnt := int64(0)
	for _, torrent := range torrents {
		fmt.Printf("Edit torrent %s (%s) tracker\n", torrent.InfoHash, torrent.Name)
		if dryRun {
			continue
		}
		err := clientInstance.EditTorrentTracker(torrent.InfoHash, oldTracker, newTracker, replaceHost)
		if err != nil {
			log.Errorf("Failed to edit tracker: %v\n", err)
			errorCnt++
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
