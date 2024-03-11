package export

import (
	"fmt"
	"os"
	"path"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/util/helper"
	"github.com/sagan/ptool/util/torrentutil"
)

var command = &cobra.Command{
	Use:         "export {client} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "export"},
	Short:       "Export and download torrents of client to .torrent files.",
	Long: `Export and download torrents of client to .torrent files.
[infoHash]...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done, _undone, _downloading, _seeding, _paused, _completed, _error.
Specially, use a single "-" as args to read infoHash list from stdin, delimited by blanks.

To set the filenames of downloaded torrents, use --rename <name> flag,
which supports the following variable placeholders:
* [size] : Torrent size
* [infohash] :  Torrent infohash
* [infohash16] :  The first 16 chars of torrent infohash
* [category] : Torrent category
* [name] : Torrent name
* [name128] : The prefix of torrent name which is at max 128 bytes

If --use-comment-meta flag is set, ptool will export torrent's current category & tags & savePath meta info,
and save them to the 'comment' field of exported .torrent file in JSON format ('{tags, category, save_path}').
The "ptool add" command has the same flag that extracts and applies meta info from 'comment' when adding torrents.

It will overwrite any existing file on disk with the same name.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: export,
}

var (
	useComment  = false
	category    = ""
	tag         = ""
	filter      = ""
	downloadDir = ""
	rename      = ""
)

func init() {
	command.Flags().BoolVarP(&useComment, "use-comment-meta", "", false,
		"Export torrent category, tags, save path and other infos to 'comment' field of .torrent file")
	command.Flags().StringVarP(&filter, "filter", "", "", "Filter torrents by name")
	command.Flags().StringVarP(&category, "category", "", "", "Filter torrents by category")
	command.Flags().StringVarP(&tag, "tag", "", "",
		"Filter torrents by tag. Comma-separated list. Torrent which tags contain any one in the list matches")
	command.Flags().StringVarP(&downloadDir, "download-dir", "", ".", `Set the download dir of exported torrents`)
	command.Flags().StringVarP(&rename, "rename", "", "[name128].[infohash16].torrent",
		"Set the name of downloaded torrents (supports variables)")
	cmd.RootCmd.AddCommand(command)
}

func export(cmd *cobra.Command, args []string) error {
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
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	torrents, err := client.QueryTorrents(clientInstance, category, tag, filter, infoHashes...)
	if err != nil {
		return err
	}
	errorCnt := int64(0)
	cntAll := len(torrents)
	for i, torrent := range torrents {
		content, err := clientInstance.ExportTorrentFile(torrent.InfoHash)
		if err != nil {
			fmt.Printf("✕ %s : failed to export %s: %v (%d/%d)\n", torrent.InfoHash, torrent.Name, err, i+1, cntAll)
			errorCnt++
			continue
		}
		if useComment {
			var useCommentErr error
			if tinfo, err := torrentutil.ParseTorrent(content, 99); err != nil {
				useCommentErr = fmt.Errorf("failed to parse: %v", err)
			} else if err := tinfo.EncodeComment(&torrentutil.TorrentCommentMeta{
				Category: torrent.Category,
				Tags:     torrent.Tags,
				SavePath: torrent.SavePath,
			}); err != nil {
				useCommentErr = fmt.Errorf("failed to encode: %v", err)
			} else if data, err := tinfo.ToBytes(); err != nil {
				useCommentErr = fmt.Errorf("failed to re-generate torrent: %v", err)
			} else {
				content = data
			}
			if useCommentErr != nil {
				fmt.Printf("✕ %s : %v (%d/%d)\n", torrent.InfoHash, useCommentErr, i+1, cntAll)
				errorCnt++
				continue
			}
		}
		filepath := path.Join(downloadDir, torrentutil.RenameExportedTorrent(torrent, rename))
		if err := os.WriteFile(filepath, content, 0600); err != nil {
			fmt.Printf("✕ %s : failed to save to %s: %v (%d/%d)\n", torrent.InfoHash, filepath, err, i+1, cntAll)
			errorCnt++
		} else {
			fmt.Printf("✓ %s : saved to %s (%d/%d)\n", torrent.InfoHash, filepath, i+1, cntAll)
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
