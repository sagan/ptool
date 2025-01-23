package export

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
	"github.com/sagan/ptool/util/torrentutil"
)

const EXPOT_TORRENT_FORMAT_CONSISTENT = "{{.client}}.{{.infohash}}.torrent"

var command = &cobra.Command{
	Use:         "export {client} [--category category] [--tag tag] [--filter filter] [infoHash]...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "export"},
	Short:       "Export and download torrents of client to .torrent files.",
	Long: fmt.Sprintf(`Export and download torrents of client to .torrent files.
%s.

To set the filename format of exported torrent, use "--rename string" flag,
which is parsed using Go text template ( https://pkg.go.dev/text/template ).
You can use all Sprig ( https://github.com/Masterminds/sprig ) functions in template.
It supports the following variables:
* client : Client name
* size : Torrent contents size string (e.g. "42GiB")
* infohash :  Torrent infohash
* infohash16 :  The first 16 chars of torrent infohash
* category : Torrent category
* name : Torrent name
* name128 : The prefix of torrent name which is at max 128 bytes
The default format is %q, unless "--skip-existing" flag is set,
in which case it's %q.

If --use-comment-meta flag is set, ptool will export torrent's current category & tags & savePath meta info,
and save them to the 'comment' field of exported .torrent file in JSON '{tags, category, save_path, comment}' format.
The "ptool add" command has the same flag that extracts and applies meta info from 'comment' when adding torrents.

It will overwrite any existing file on disk with the same name.`,
		constants.HELP_INFOHASH_ARGS, config.DEFAULT_EXPORT_TORRENT_RENAME, EXPOT_TORRENT_FORMAT_CONSISTENT),
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
			`If this flag is set, the exported torrent filename format ("--rename" flag) will be fixed to `+
			`"`+EXPOT_TORRENT_FORMAT_CONSISTENT+`"`)
	command.Flags().BoolVarP(&useCommentMeta, "use-comment-meta", "", false,
		`Export torrent category, tags, save path and other infos to "comment" field of .torrent file`)
	command.Flags().StringVarP(&filter, "filter", "", "", constants.HELP_ARG_FILTER_TORRENT)
	command.Flags().StringVarP(&category, "category", "", "", constants.HELP_ARG_CATEGORY)
	command.Flags().StringVarP(&tag, "tag", "", "", constants.HELP_ARG_TAG)
	command.Flags().StringVarP(&downloadDir, "download-dir", "", ".", `Set the download dir of exported torrents. `+
		`Set to "-" to output to stdout`)
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
	renameTemplate, err := template.New("template").Funcs(sprig.FuncMap()).Parse(rename)
	if err != nil {
		return fmt.Errorf("invalid rename template: %v", err)
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	torrents, err := client.QueryTorrents(clientInstance, category, tag, filter, infoHashes...)
	if err != nil {
		return err
	}
	if downloadDir == "-" {
		if len(torrents) > 1 {
			return fmt.Errorf(`only 1 (one) torrent can be outputted to stdout`)
		}
		if term.IsTerminal(int(os.Stdout.Fd())) {
			return fmt.Errorf(constants.HELP_TIP_TTY_BINARY_OUTPUT)
		}
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
			name, err := torrentutil.RenameExportedTorrent(clientName, torrent, renameTemplate)
			if err != nil {
				fmt.Printf("✕ %s : %v (%d/%d)\n", torrent.InfoHash, err, i+1, cntAll)
				errorCnt++
				continue
			}
			filename = name
		}
		outputPath := ""
		if downloadDir == "-" {
			outputPath = downloadDir
		} else {
			outputPath = filepath.Join(downloadDir, filename)
		}
		_, _, err := common.ExportClientTorrent(clientInstance, torrent, outputPath, useCommentMeta)
		if err != nil {
			fmt.Printf("✕ %s : %v (%d/%d)\n", torrent.InfoHash, err, i+1, cntAll)
			errorCnt++
		} else {
			fmt.Printf("✓ %s : saved to %s (%d/%d)\n", torrent.InfoHash, outputPath, i+1, cntAll)
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
