package export

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
	"github.com/sagan/ptool/util/torrentutil"
)

var command = &cobra.Command{
	Use:         "export {client} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "export"},
	Short:       "Export and download torrents of client to .torrent files.",
	Long: fmt.Sprintf(`Export and download torrents of client to .torrent files.
%s.

To set the filename of downloaded torrent, use --rename <name> flag,
which supports the following variable placeholders:
* [client] : Client name
* [size] : Torrent size
* [infohash] :  Torrent infohash
* [infohash16] :  The first 16 chars of torrent infohash
* [category] : Torrent category
* [name] : Torrent name
* [name128] : The prefix of torrent name which is at max 128 bytes

If --use-comment-meta flag is set, ptool will export torrent's current category & tags & savePath meta info,
and save them to the 'comment' field of exported .torrent file in JSON '{tags, category, save_path, comment}' format.
The "ptool add" command has the same flag that extracts and applies meta info from 'comment' when adding torrents.

It will overwrite any existing file on disk with the same name.`, constants.HELP_INFOHASH_ARGS),
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: export,
}

var (
	skipExisting   = false
	useCommentMeta = false
	category       = ""
	tag            = ""
	filter         = ""
	downloadDir    = ""
	rename         = ""
)

func init() {
	command.Flags().BoolVarP(&skipExisting, "skip-existing", "", false,
		`Do NOT re-export torrent that same name file already exists in local dir. `+
			`If this flag is set, the exported torrent filename ("--rename" flag) will be fixed to `+
			`"[client].[infohash].torrent" (e.g. "local.293235f712652df08a8665ec2ca118d7e0615c3f.torrent") format`)
	command.Flags().BoolVarP(&useCommentMeta, "use-comment-meta", "", false,
		`Export torrent category, tags, save path and other infos to "comment" field of .torrent file`)
	command.Flags().StringVarP(&filter, "filter", "", "", constants.HELP_ARG_FILTER_TORRENT)
	command.Flags().StringVarP(&category, "category", "", "", constants.HELP_ARG_CATEGORY)
	command.Flags().StringVarP(&tag, "tag", "", "", constants.HELP_ARG_TAG)
	command.Flags().StringVarP(&downloadDir, "download-dir", "", ".", `Set the download dir of exported torrents`)
	command.Flags().StringVarP(&rename, "rename", "", config.DEFAULT_EXPORT_TORRENT_RENAME,
		"Set the name of downloaded torrents (supports variables)")
	cmd.RootCmd.AddCommand(command)
}

func export(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	infoHashes := args[1:]
	if category == "" && tag == "" && filter == "" {
		if _infoHashes, err := helper.ParseInfoHashesFromArgs(infoHashes); err != nil {
			return err
		} else {
			infoHashes = _infoHashes
		}
	}
	if skipExisting && rename != config.DEFAULT_EXPORT_TORRENT_RENAME {
		return fmt.Errorf("--skip-existing and --rename flags are NOT compatible")
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	torrents, err := client.QueryTorrents(clientInstance, category, tag, filter, infoHashes...)
	if err != nil {
		return err
	}
	errorCnt := int64(0)
	cntAll := len(torrents)
	for i, torrent := range torrents {
		filename := ""
		if skipExisting {
			filename = fmt.Sprintf("%s.%s.torrent", clientName, torrent.InfoHash)
			if util.FileExistsWithOptionalSuffix(filepath.Join(downloadDir, filename),
				constants.ProcessedFilenameSuffixes...) {
				fmt.Printf("- %s : skip local existing torrent (%d/%d)\n", torrent.InfoHash, i+1, cntAll)
				continue
			}
		} else {
			filename = torrentutil.RenameExportedTorrent(clientName, torrent, rename)
		}
		filepath := filepath.Join(downloadDir, filename)
		_, _, err := common.ExportClientTorrent(clientInstance, torrent, filepath, useCommentMeta)
		if err != nil {
			fmt.Printf("✕ %s : %v (%d/%d)\n", torrent.InfoHash, err, i+1, cntAll)
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
