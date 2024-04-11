package addtrackers

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use:         "addtrackers {client} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Aliases:     []string{"addtracker"},
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "addtrackers"},
	Short:       "Add new trackers to torrents of client.",
	Long: `Add new trackers to torrents of client.
[infoHash]...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done, _undone, _downloading, _seeding, _paused, _completed, _error.
Specially, use a single "-" as args to read infoHash list from stdin, delimited by blanks.

Example:
ptool addtrackers <client> <infoHashes...> --tracker "https://..."
The --tracker flag can be set many times.

It will ask for confirmation, unless --force flag is set.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: addtrackers,
}

var (
	force      = false
	category   = ""
	tag        = ""
	filter     = ""
	oldTracker = ""
	trackers   = []string{}
)

func init() {
	command.Flags().BoolVarP(&force, "force", "", false, "Force updating trackers. Do NOT prompt for confirm")
	command.Flags().StringVarP(&filter, "filter", "", "", "Filter torrents by name")
	command.Flags().StringVarP(&category, "category", "", "", "Filter torrents by category")
	command.Flags().StringVarP(&tag, "tag", "", "",
		"Filter torrents by tag. Comma-separated list. Torrent which tags contain any one in the list matches")
	command.Flags().StringVarP(&oldTracker, "old-tracker", "", "",
		"The existing tracker host or url. If set, only torrents that already have this tracker will get new tracker")
	command.Flags().StringArrayVarP(&trackers, "tracker", "", nil, "Set the tracker to add. Can be set multiple times")
	command.MarkFlagRequired("tracker")
	cmd.RootCmd.AddCommand(command)
}

func addtrackers(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	infoHashes := args[1:]
	if category == "" && tag == "" && filter == "" {
		if len(infoHashes) == 0 {
			return fmt.Errorf("you must provide at least a condition flag or hashFilter")
		}
		if len(infoHashes) == 1 && infoHashes[0] == "-" {
			if data, err := helper.ReadArgsFromStdin(); err != nil {
				return fmt.Errorf("failed to parse stdin to info hashes: %v", err)
			} else if len(data) == 0 {
				return nil
			} else {
				infoHashes = data
			}
		}
	}
	if len(trackers) == 0 {
		return fmt.Errorf("at least one --tracker flag must be set")
	}
	for _, tracker := range trackers {
		if !util.IsUrl(tracker) {
			return fmt.Errorf("the provided tracker %s is not a valid URL", tracker)
		}
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	torrents, err := client.QueryTorrents(clientInstance, category, tag, filter, infoHashes...)
	if err != nil {
		return err
	}
	if len(torrents) == 0 {
		log.Infof("No matched torrents found")
		return nil
	}
	if !force {
		client.PrintTorrents(torrents, "", 1, false)
		fmt.Printf("\n")
		if !util.AskYesNoConfirm(fmt.Sprintf(
			`Will update above %d torrents, add the following trackers to them:
-----
%s
-----
`, len(torrents), strings.Join(trackers, "\n"))) {
			return fmt.Errorf("abort")
		}
	}
	errorCnt := int64(0)
	for _, torrent := range torrents {
		fmt.Printf("Add trackers to torrent %s (%s)\n", torrent.InfoHash, torrent.Name)
		err := clientInstance.AddTorrentTrackers(torrent.InfoHash, trackers, oldTracker)
		if err != nil {
			log.Errorf("Failed to add trackers: %v\n", err)
			errorCnt++
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
