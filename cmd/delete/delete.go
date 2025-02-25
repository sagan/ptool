package delete

import (
	"fmt"
	"os"

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
	showSum           = false
	preserve          = false
	preserveXseed     = false
	force             = false
	filter            = ""
	category          = ""
	tag               = ""
	tracker           = ""
	minTorrentSizeStr = ""
	maxTorrentSizeStr = ""
)

func init() {
	command.Flags().BoolVarP(&showSum, "sum", "", false, "Show torrents summary only")
	command.Flags().BoolVarP(&preserve, "preserve", "p", false,
		"Preserve (don't delete) torrent content files on the disk")
	command.Flags().BoolVarP(&preserveXseed, "preserve-if-xseed-exist", "P", false,
		"Preserve (don't delete) torrent content files on the disk if other xseed torrents exist")
	command.Flags().BoolVarP(&force, "force", "", false, "Force deletion. Do NOT prompt for confirm")
	command.Flags().StringVarP(&filter, "filter", "", "", constants.HELP_ARG_FILTER_TORRENT)
	command.Flags().StringVarP(&category, "category", "", "", constants.HELP_ARG_CATEGORY)
	command.Flags().StringVarP(&tag, "tag", "", "", constants.HELP_ARG_TAG)
	command.Flags().StringVarP(&tracker, "tracker", "", "", constants.HELP_ARG_TRACKER)
	command.Flags().StringVarP(&minTorrentSizeStr, "min-torrent-size", "", "-1", constants.HELP_ARG_MIN_TORRENT_SIZE)
	command.Flags().StringVarP(&maxTorrentSizeStr, "max-torrent-size", "", "-1", constants.HELP_ARG_MAX_TORRENT_SIZE)
	cmd.RootCmd.AddCommand(command)
}

func delete(cmd *cobra.Command, args []string) error {
	if preserve && preserveXseed {
		return fmt.Errorf("--preserve and --preserve-if-xseed-exist flags are NOT compatible")
	}
	clientName := args[0]
	infoHashes := args[1:]
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
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

	// the quick way, directly submit the deletion request to client
	if infohashesOnly && !preserveXseed {
		if len(infoHashes) == 0 {
			return fmt.Errorf("no torrent to delete")
		}
		if force {
			if err = clientInstance.DeleteTorrents(infoHashes, !preserve); err != nil {
				return fmt.Errorf("failed to delete torrents: %w", err)
			}
			return nil
		}
	}
	torrents, err := client.QueryTorrents(clientInstance, category, tag, filter, infoHashes...)
	if err != nil {
		return fmt.Errorf("failed to fetch client torrents: %w", err)
	}
	if tracker != "" || minTorrentSize >= 0 || maxTorrentSize >= 0 {
		torrents = util.Filter(torrents, func(t *client.Torrent) bool {
			if tracker != "" && !t.MatchTracker(tracker) ||
				minTorrentSize >= 0 && t.Size < minTorrentSize ||
				maxTorrentSize >= 0 && t.Size > maxTorrentSize {
				return false
			}
			return true
		})
	}
	// if preserve-xseed flag is set, the torrents which contains other-not-delete xseed torrents
	var torrentsWithXseed []*client.Torrent
	if preserveXseed {
		torrents, torrentsWithXseed, err = client.FilterTorrentsXseed(clientInstance, torrents)
		if err != nil {
			return err
		}
	}
	if len(torrents) == 0 && len(torrentsWithXseed) == 0 {
		log.Infof("No matched torrents found")
		return nil
	}
	if !force {
		if len(torrents) > 0 {
			sum := int64(1)
			if showSum {
				sum = 2
			}
			client.PrintTorrents(os.Stdout, torrents, "", sum, false)
			fmt.Printf("Above %d torrents will be deteled (Delete disk files = %t)\n", len(torrents), !preserve)
			fmt.Printf("\n")
		}
		if len(torrentsWithXseed) > 0 {
			client.PrintTorrents(os.Stdout, torrentsWithXseed, "", 1, false)
			fmt.Printf("Above %d torrents will be deleted, they have non-delete xseed torrents exists,\n"+
				"so their disk files will NOT be deleted.\n", len(torrentsWithXseed))
			fmt.Printf("\n")
		}
		if !helper.AskYesNoConfirm("") {
			return fmt.Errorf("abort")
		}
	}
	if len(torrentsWithXseed) > 0 {
		infoHashes := util.Map(torrentsWithXseed, func(t *client.Torrent) string { return t.InfoHash })
		err = clientInstance.DeleteTorrents(infoHashes, false)
		if err != nil {
			return fmt.Errorf("failed to delete torrents: %w", err)
		}
		fmt.Printf("%d torrents deleted (delete files = false).\n", len(torrentsWithXseed))
	}
	if len(torrents) > 0 {
		infoHashes := util.Map(torrents, func(t *client.Torrent) string { return t.InfoHash })
		err = clientInstance.DeleteTorrents(infoHashes, !preserve)
		if err != nil {
			return fmt.Errorf("failed to delete torrents: %w", err)
		}
		fmt.Printf("%d torrents deleted (delete files = %t).\n", len(torrents), !preserve)
	}
	return nil
}
