package markinvalidtracker

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/client/transmission"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use:         "markinvalidtracker {client} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "markinvalidtracker"},
	Aliases:     []string{"markinvalid"},
	Short:       fmt.Sprintf(`Mark torrents in client with a invalid tracker %q tag.`, config.INVALID_TRACKER_TAG),
	Long: fmt.Sprintf(`Mark torrents in client with a invalid tracker %q tag.
%s.

It will check tracker status of torrents in client, mark those torrents which
trackers status is invalid with %q tag.

A torrent's trackers status is treated as invalid if any of the following condition is true:
- Torrent is not registered in the tracker(s).
- Passkey or authkey is required or invalid.

If "--all" flag is set, it also treats any of following conditions as invalid tracker:
- It's exceeding the simultaneous downloading / seeding clients number limit.

A torrent's trackers status is NOT treated as invalid if the tracker(s)
is not accessible due to network problems or site server error.

Note it will first reset %q tag, removing all torrents from it, before adding torrents to it`,
		config.INVALID_TRACKER_TAG, constants.HELP_INFOHASH_ARGS, config.INVALID_TRACKER_TAG, config.INVALID_TRACKER_TAG),
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: markinvalidtracker,
}

var (
	noClean  = false
	all      = false
	category = ""
	tag      = ""
	filter   = ""
)

func init() {
	command.Flags().BoolVarP(&all, "all", "a", false,
		"Include torrents which are exceeding the simultaneous downloading / seeding clients number limit")
	command.Flags().StringVarP(&filter, "filter", "", "", constants.HELP_ARG_FILTER_TORRENT)
	command.Flags().StringVarP(&category, "category", "", "", constants.HELP_ARG_CATEGORY)
	command.Flags().BoolVarP(&noClean, "no-clean", "", false, `Do not clean existing torrents of "`+
		config.INVALID_TRACKER_TAG+`" tag`)
	command.Flags().StringVarP(&tag, "tag", "", "", constants.HELP_ARG_TAG)
	cmd.RootCmd.AddCommand(command)
}

func markinvalidtracker(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	infoHashes := args[1:]
	if category == "" && tag == "" && filter == "" {
		if _infoHashes, err := helper.ParseInfoHashesFromArgs(infoHashes); err != nil {
			return err
		} else {
			infoHashes = _infoHashes
		}
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// A workaround for transmission performance boost. tr can get all infos in batch
	if trClient, ok := clientInstance.(*transmission.Client); ok {
		trClient.Sync(true)
	}
	torrents, err := client.QueryTorrents(clientInstance, category, tag, filter, infoHashes...)
	if err != nil {
		return fmt.Errorf("failed to query client torrents: %w", err)
	}
	errorCnt := int64(0)
	infoHashes = nil
	for _, torrent := range torrents {
		log.Debugf("Check %s (%s) trackers status...", torrent.InfoHash, torrent.Name)
		trackers, err := clientInstance.GetTorrentTrackers(torrent.InfoHash)
		if err != nil {
			log.Errorf("Failed to get torrent %s trackers: %v", torrent.InfoHash, err)
			errorCnt++
			continue
		}
		if !trackers.SeemsInvalidTorrent(all) {
			continue
		}
		log.Warnf("torrent %s (%s)'s trackers seems invalid: %v\n", torrent.InfoHash, torrent.Name, trackers)
		infoHashes = append(infoHashes, torrent.InfoHash)
	}
	log.Warnf("Marking %d torrents as invalid tracker", len(infoHashes))
	if !noClean {
		if err = clientInstance.DeleteTags(config.INVALID_TRACKER_TAG); err != nil {
			return fmt.Errorf("failed to clean mark tag: %w", err)
		}
	}
	if len(infoHashes) > 0 {
		if err = clientInstance.AddTagsToTorrents(infoHashes, []string{config.INVALID_TRACKER_TAG}); err != nil {
			return fmt.Errorf("failed to mark invalid tracker torrents: %w", err)
		}
		fmt.Printf("Found %d torrents with invalid tracker, marked them with %q tag\n",
			len(infoHashes), config.INVALID_TRACKER_TAG)
	} else {
		fmt.Printf("No invalid tracker torrent found\n")
	}

	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
