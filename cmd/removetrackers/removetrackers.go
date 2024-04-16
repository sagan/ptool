package removetrackers

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use:         "removetrackers {client} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Aliases:     []string{"removetracker"},
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "removetrackers"},
	Short:       "Remove trackers from torrents of client.",
	Long: fmt.Sprintf(`Remove trackers from torrents of client.
%s.

Example:
ptool removetrackers <client> <infoHashes...> --tracker "https://..."
The --tracker flag can be set many times.

It will ask for confirmation, unless --force flag is set.`, constants.HELP_INFOHASH_ARGS),
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: removetrackers,
}

var (
	force    = false
	category = ""
	tag      = ""
	filter   = ""
	trackers = []string{}
)

func init() {
	command.Flags().BoolVarP(&force, "force", "", false, "Force updating trackers. Do NOT prompt for confirm")
	command.Flags().StringVarP(&filter, "filter", "", "", "Filter torrents by name")
	command.Flags().StringVarP(&category, "category", "", "", "Filter torrents by category")
	command.Flags().StringVarP(&tag, "tag", "", "",
		"Filter torrents by tag. Comma-separated list. Torrent which tags contain any one in the list matches")
	command.Flags().StringArrayVarP(&trackers, "tracker", "", nil,
		"Set the tracker to remove. Can be set multiple times")
	command.MarkFlagRequired("tracker")
	cmd.RootCmd.AddCommand(command)
}

func removetrackers(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	infoHashes := args[1:]
	if category == "" && tag == "" && filter == "" {
		if _infoHashes, err := helper.ParseInfoHashesFromArgs(infoHashes); err != nil {
			return err
		} else {
			infoHashes = _infoHashes
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
		if !helper.AskYesNoConfirm(fmt.Sprintf(
			`Will update above %d torrents, remove the following trackers from them:
-----
%s
-----
`, len(torrents), strings.Join(trackers, "\n"))) {
			return fmt.Errorf("abort")
		}
	}
	errorCnt := int64(0)
	for _, torrent := range torrents {
		fmt.Printf("Remove trackers from torrent %s (%s)\n", torrent.InfoHash, torrent.Name)
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
