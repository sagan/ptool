package transfertorrent

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use: "transfertorrent {src-client} --dst-client {dst-client} " +
		"[--category category] [--tag tag] [--filter filter] [infoHash]...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "transfertorrent"},
	Aliases:     []string{"transfer"},
	Short:       `Transfer torrents of client to another client.`,
	Long: fmt.Sprintf(`Transfer torrents of client to another client.
%s.

{src-client} and {dst-client} shoud be in the same machine.
If they have different file systems, use "--map-save-path" flag to the the path mapper rule.

Only torrents in {src-client} that is fullly completed downloaded will be transferred.
It will mark successfully moved torrents in {src-client} with %q tag. However, it won't
delete these torrents from {src-client} after transferred them, to do it, you need to run:
  ptool delete {src-client} --tag %q --preserve

Due to technonicol limitations, {src-client} must be qBittorrent at this time.`,
		constants.HELP_INFOHASH_ARGS, config.TRANSFERRED_TAG, config.TRANSFERRED_TAG),
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: transfertorrent,
}

var (
	addPaused    bool
	force        bool
	maxTorrents  int64
	category     string
	tag          string
	filter       string
	dstClient    string
	mapSavePaths []string
)

func init() {
	command.Flags().BoolVarP(&addPaused, "add-paused", "", false, "Add torrents to dst client in paused state")
	command.Flags().BoolVarP(&force, "force", "", false, "Do transfer torrents without confirm")
	command.Flags().Int64VarP(&maxTorrents, "max-torrents", "", -1,
		"Number limit of transferred torrents. -1 == no limit")
	command.Flags().StringVarP(&filter, "filter", "", "", constants.HELP_ARG_FILTER_TORRENT)
	command.Flags().StringVarP(&category, "category", "", "", constants.HELP_ARG_CATEGORY)
	command.Flags().StringVarP(&tag, "tag", "", "", constants.HELP_ARG_TAG)
	command.Flags().StringVarP(&dstClient, "dst-client", "", "", `Target client. Transfer torrents to this client`)
	command.Flags().StringArrayVarP(&mapSavePaths, "map-save-path", "", nil,
		`Used with "--use-comment-meta". Map save path from source BitTorrent client to dest BitTorrent client.`+
			`Format: "src_client_path|dst_client_path". `+constants.HELP_ARG_PATH_MAPPERS)
	command.MarkFlagRequired("dst-client")
	cmd.RootCmd.AddCommand(command)
}

func transfertorrent(cmd *cobra.Command, args []string) (err error) {
	srcClient := args[0]
	infoHashes := args[1:]
	if category == "" && tag == "" && filter == "" {
		if _infoHashes, err := helper.ParseInfoHashesFromArgs(infoHashes); err != nil {
			return err
		} else {
			infoHashes = _infoHashes
		}
	}
	var savePathMapper *common.PathMapper
	if len(mapSavePaths) > 0 {
		savePathMapper, err = common.NewPathMapper(mapSavePaths)
		if err != nil {
			return fmt.Errorf("invalid map-save-path(s): %w", err)
		}
	}
	srcClientInstance, err := client.CreateClient(srcClient)
	if err != nil {
		return fmt.Errorf("failed to create src client: %w", err)
	}
	dstClientInstance, err := client.CreateClient(dstClient)
	if err != nil {
		return fmt.Errorf("failed to create dst client: %w", err)
	}

	torrents, err := client.QueryTorrents(srcClientInstance, category, tag, filter, infoHashes...)
	if err != nil {
		return fmt.Errorf("failed to query client torrents: %w", err)
	}
	if len(torrents) == 0 {
		fmt.Printf("no torrents to move")
		return nil
	}

	if !force {
		fmt.Printf(`Will move %d torrents from %s to %s
Add torrents to target client in paused state: %t`+"\n", len(torrents), srcClient, dstClient, addPaused)
		if savePathMapper != nil {
			fmt.Printf("Save path map rules (src_client_path|dst_client_path): %v\n", mapSavePaths)
		}
		if !helper.AskYesNoConfirm("") {
			return fmt.Errorf("abort")
		}
	}

	errorCnt := int64(0)
	var movedInfoHashes []string
	for i, torrent := range torrents {
		fmt.Printf("(%d/%d) ", i+1, len(torrents))
		if !torrent.IsFullComplete() {
			fmt.Printf("! %s (%s): do not transfer due is not fully downloaded completed\n", torrent.InfoHash, torrent.Name)
			continue
		}
		if torrent.HasTag(config.TRANSFERRED_TAG) {
			fmt.Printf("- %s (%s): already transferred before, to re-do, remove %s tag from it\n",
				torrent.InfoHash, torrent.Name, config.TRANSFERRED_TAG)
			continue
		}
		targetpapth := torrent.SavePath
		if savePathMapper != nil {
			newpath, match := savePathMapper.Before2After(torrent.SavePath)
			if !match {
				fmt.Printf("! %s (%s): do not move due to save path cann't be mapped\n", torrent.InfoHash, torrent.Name)
				continue
			}
			targetpapth = newpath
		}
		torrentContent, err := srcClientInstance.ExportTorrentFile(torrent.InfoHash)
		if err != nil {
			fmt.Printf("✕ %s (%s): failed to export torrent: %v\n", torrent.InfoHash, torrent.Name, err)
			errorCnt++
			continue
		}
		err = dstClientInstance.AddTorrent(torrentContent, &client.TorrentOption{
			Category:     torrent.Category,
			Tags:         torrent.Tags,
			SkipChecking: true,
			SavePath:     targetpapth,
			Pause:        addPaused,
		}, nil)
		if err != nil {
			fmt.Printf("✕ %s (%s): failed to move to target client: %v\n", torrent.InfoHash, torrent.Name, err)
			errorCnt++
			continue
		}
		fmt.Printf("✓ %s (%s): moved to target client, save path: %q\n", torrent.InfoHash, torrent.Name, targetpapth)
		movedInfoHashes = append(movedInfoHashes, torrent.InfoHash)
		if maxTorrents > 0 && int64(len(movedInfoHashes)) >= maxTorrents {
			break
		}
	}

	fmt.Printf("Transfer %d torrents to target client\n", len(movedInfoHashes))
	if len(movedInfoHashes) > 0 {
		err = srcClientInstance.AddTagsToTorrents(movedInfoHashes, []string{config.TRANSFERRED_TAG})
		if err != nil {
			fmt.Printf("Failed to mark moved torrent in srcClient, do it youself. torrents: %v\n", movedInfoHashes)
			return err
		}
	}

	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
