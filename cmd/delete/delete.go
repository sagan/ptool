package delete

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use:         "delete {client} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "delete"},
	Aliases:     []string{"del", "rm"},
	Short:       "Delete torrents from client.",
	Long: fmt.Sprintf(`Delete torrents from client.
%s.

It will ask for confirmation of deletion, unless --force flag is set.`, constants.HELP_INFOHASH_ARGS),
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: delete,
}

var (
	preserve          = false
	force             = false
	filter            = ""
	category          = ""
	tag               = ""
	tracker           = ""
	minTorrentSizeStr = ""
	maxTorrentSizeStr = ""
)

func init() {
	command.Flags().BoolVarP(&preserve, "preserve", "p", false,
		"Preserve (don't delete) torrent content files on the disk")
	command.Flags().BoolVarP(&force, "force", "", false, "Force deletion. Do NOT prompt for confirm")
	command.Flags().StringVarP(&filter, "filter", "", "", constants.HELP_ARG_FILTER_TORRENT)
	command.Flags().StringVarP(&category, "category", "", "", constants.HELP_ARG_CATEGORY)
	command.Flags().StringVarP(&tag, "tag", "", "", constants.HELP_ARG_TAG)
	command.Flags().StringVarP(&tracker, "tracker", "", "", constants.HELP_ARG_TRACKER)
	command.Flags().StringVarP(&minTorrentSizeStr, "min-torrent-size", "", "-1",
		"Skip torrent with size smaller than (<) this value. -1 == no limit")
	command.Flags().StringVarP(&maxTorrentSizeStr, "max-torrent-size", "", "-1",
		"Skip torrent with size larger than (>) this value. -1 == no limit")
	cmd.RootCmd.AddCommand(command)
}

func delete(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	infoHashes := args[1:]
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}
	minTorrentSize, _ := util.RAMInBytes(minTorrentSizeStr)
	maxTorrentSize, _ := util.RAMInBytes(maxTorrentSizeStr)
	infohashesOnly := true
	if category != "" || tag != "" || filter != "" || tracker != "" || minTorrentSize >= 0 || maxTorrentSize >= 0 {
		infohashesOnly = false
	} else {
		if _infoHashes, err := helper.ParseInfoHashesFromArgs(infoHashes); err != nil {
			return err
		} else {
			infoHashes = _infoHashes
		}
		for _, infoHash := range infoHashes {
			if !client.IsValidInfoHash(infoHash) {
				infohashesOnly = false
				break
			}
		}
	}

	if infohashesOnly {
		if len(infoHashes) == 0 {
			return fmt.Errorf("no torrent to delete")
		}
		if force {
			if err = clientInstance.DeleteTorrents(infoHashes, !preserve); err != nil {
				return fmt.Errorf("failed to delete torrents: %v", err)
			}
			return nil
		}
	}
	torrents, err := client.QueryTorrents(clientInstance, category, tag, filter, infoHashes...)
	if err != nil {
		return fmt.Errorf("failed to fetch client torrents: %v", err)
	}
	if tracker != "" || minTorrentSize >= 0 || maxTorrentSize >= 0 {
		torrents = util.Filter(torrents, func(t client.Torrent) bool {
			if tracker != "" && !t.MatchTracker(tracker) ||
				minTorrentSize >= 0 && t.Size < minTorrentSize ||
				maxTorrentSize >= 0 && t.Size > maxTorrentSize {
				return false
			}
			return true
		})
	}
	if len(torrents) == 0 {
		log.Infof("No matched torrents found")
		return nil
	}
	if !force {
		client.PrintTorrents(torrents, "", 1, false)
		fmt.Printf("\n")
		if !helper.AskYesNoConfirm(fmt.Sprintf("Above %d torrents will be deteled (Preserve disk files = %t)",
			len(torrents), preserve)) {
			return fmt.Errorf("abort")
		}
	}
	infoHashes = util.Map(torrents, func(t client.Torrent) string { return t.InfoHash })
	err = clientInstance.DeleteTorrents(infoHashes, !preserve)
	if err != nil {
		return fmt.Errorf("failed to delete torrents: %v", err)
	}
	fmt.Printf("%d torrents deleted.\n", len(torrents))
	return nil
}
