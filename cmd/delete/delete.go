package delete

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/util"
)

var command = &cobra.Command{
	Use:         "delete {client} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "delete"},
	Aliases:     []string{"rm"},
	Short:       "Delete torrents from client.",
	Long: `Delete torrents from client.
[infoHash]...: infoHash list of torrents to delete. It's possible to use state filter to target multiple torrents:
_all, _active, _done, _undone, _downloading, _seeding, _paused, _completed, _error.
Specially, use a single "-" as args to read infoHash list from stdin, delimited by blanks.

It will ask for confirmation of deletion, unless --force flag is set.`,
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
	command.Flags().StringVarP(&filter, "filter", "", "", "Filter torrents by name")
	command.Flags().StringVarP(&category, "category", "", "", "Filter torrents by category")
	command.Flags().StringVarP(&tag, "tag", "", "",
		"Filter torrents by tag. Comma-separated list. Torrent which tags contain any one in the list matches")
	command.Flags().StringVarP(&tracker, "tracker", "", "", "Filter torrents by tracker domain")
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
		// special case. read info hashes from stdin
		if len(infoHashes) == 1 && infoHashes[0] == "-" {
			if config.InShell {
				return fmt.Errorf(`"-" arg can not be used in shell`)
			}
			if data, err := util.ReadArgsFromStdin(); err != nil {
				return fmt.Errorf("failed to parse stdin to info hashes: %v", err)
			} else if len(data) == 0 {
				return nil
			} else {
				infoHashes = data
			}
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
			if tracker != "" && t.TrackerDomain != tracker ||
				minTorrentSize >= 0 && t.Size < minTorrentSize ||
				maxTorrentSize >= 0 && t.Size > maxTorrentSize {
				return false
			}
			return true
		})
	}
	if !force {
		client.PrintTorrents(torrents, "", 1, false)
		fmt.Printf("\n")
		if !util.AskYesNoConfirm(fmt.Sprintf("Above %d torrents will be deteled (Preserve disk files = %t)",
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
