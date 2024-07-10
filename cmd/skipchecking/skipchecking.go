package skipchecking

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use:         "skipchecking {client} {info-hash}...",
	Aliases:     []string{"skipcheck"},
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "skipchecking"},
	Short:       "Skip checking torrents in BitTorrent client",
	Long: fmt.Sprintf(`Skip checking torrents in BitTorrent client.
	%s.

It works in the following procedures:

1. Export torrent in client which is currently in checking state.
2. Delete exported torrents from client (torrent content files in disk are NOT deleted).
3. Add back exported torrents to client and skip checking.`,
		constants.HELP_INFOHASH_ARGS),
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: skipchecking,
}

var (
	category = ""
	tag      = ""
	filter   = ""
)

func init() {
	command.Flags().StringVarP(&filter, "filter", "", "", constants.HELP_ARG_FILTER_TORRENT)
	command.Flags().StringVarP(&category, "category", "", "", constants.HELP_ARG_CATEGORY)
	command.Flags().StringVarP(&tag, "tag", "", "", constants.HELP_ARG_TAG)
	cmd.RootCmd.AddCommand(command)
}

func skipchecking(cmd *cobra.Command, args []string) (err error) {
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
	torrents, err := client.QueryTorrents(clientInstance, category, tag, filter, infoHashes...)
	if err != nil {
		return fmt.Errorf("failed to query client torrents: %w", err)
	}
	tmppath := filepath.Join(config.ConfigDir, "sc")
	if err = os.MkdirAll(tmppath, constants.PERM_DIR); err != nil {
		return fmt.Errorf("failed to create tmp dir %q: %w", tmppath, err)
	}

	skipped := int64(0)
	for _, torrent := range torrents {
		if torrent.State != "checking" || torrent.Ctime == 0 {
			continue
		}
		filepath := filepath.Join(tmppath, clientName+"."+torrent.InfoHash+".torrent")
		contents, _, err := common.ExportClientTorrent(clientInstance, torrent, filepath, true)
		if err != nil {
			return fmt.Errorf("failed to export torrent %s (%s): %v", torrent.Name, torrent.InfoHash, err)
		}
		fmt.Printf("Skip %s (%s) (%s)\n", torrent.Name, torrent.InfoHash, filepath)
		if err = clientInstance.DeleteTorrents([]string{torrent.InfoHash}, false); err != nil {
			return fmt.Errorf("failed to delete torrent from client: %v", err)
		}
		err = clientInstance.AddTorrent(contents, &client.TorrentOption{
			SavePath:     torrent.SavePath,
			Category:     torrent.Category,
			Tags:         torrent.Tags,
			SkipChecking: true,
		}, nil)
		if err != nil {
			return fmt.Errorf(`failed to add torrent to client: %v. Please run `+
				`"ptool add %q %q --use-comment-meta --skip-check" manually to add back torrent to client`,
				err, clientName, filepath)
		}
		skipped++
		os.Remove(filepath)
	}

	fmt.Printf("skipped checking %d torrents\n", skipped)
	return nil
}
