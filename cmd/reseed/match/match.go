package match

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd/reseed"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
	"github.com/sagan/ptool/util/torrentutil"
)

var command = &cobra.Command{
	Use:         "match <save-path>...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "reseed.match"},
	Short:       "Match contents of local disk download folders using Reseed API.",
	Long: `Match contents of local disk download folders using Reseed API.
<save-path>...: the "save path" (download location) of BitTorrent client, e.g. "./Downloads".

If run without --download flag, it just prints found (xseed) torrent ids and exit.

To download found torrents to local, use --download flag,
by default only full match torrents will be downloaded,
use --download-all flag to download ALL (include partial-match results).

By default it downloads torrents to "<config_dir>/reseed", the <config_dir> is the folder
where ptool.toml config file is located at.

Existing torrents in local (already downloaded before) will be skipped.

To add downloaded torrents to local client as xseed torrents, use "ptool xseedadd" cmd.

If --use-comment-meta flag is set, it will export <save-path> to downloaded .torrent files,
use "ptool add" cmd with same flag to directly add these torrents to local client with
their save-path in comment automatically applied.
Also, for partial-match results, it's the only way to automatically add them to local client as xseed torrent,
as "xseedadd" cmd will fail to find matched target for such torrent in client.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: match,
}

var (
	useComment  = false
	doDownload  = false
	downloadAll = false
	timeout     = int64(0)
	downloadDir = ""
)

func init() {
	command.Flags().BoolVarP(&useComment, "use-comment-meta", "", false,
		"Use with --download. Use 'comment' field to export save path location to downloaded .torrent files")
	command.Flags().BoolVarP(&doDownload, "download", "", false, "Download found xseed torrents to local")
	command.Flags().BoolVarP(&downloadAll, "download-all", "", false,
		"Use with --download. Download all found xseed torrents (include partial-match results)")
	command.Flags().Int64VarP(&timeout, "timeout", "", 15, "Timeout (seconds) for requesting Reseed API")
	command.Flags().StringVarP(&downloadDir, "download-dir", "", "",
		`Set the dir of downloaded .torrent files. By default it uses "<config_dir>/reseed"`)
	reseed.Command.AddCommand(command)
}

func match(cmd *cobra.Command, args []string) error {
	savePathes := args
	if config.Get().ReseedUsername == "" || config.Get().ReseedPassword == "" {
		return fmt.Errorf("you must config reseedUsername & reseedPassword in ptool.toml to use reseed functions")
	}
	if timeout <= 0 {
		return fmt.Errorf("timeout must be > 0")
	}
	if useComment && len(savePathes) > 1 {
		return fmt.Errorf("--use-comment-meta flag must be used with only 1 <save-path> arg")
	}
	if downloadDir == "" {
		downloadDir = filepath.Join(config.ConfigDir, "reseed")
	}
	if doDownload {
		if err := os.MkdirAll(downloadDir, 0666); err != nil {
			return fmt.Errorf("failed to create download-dir %s: %v", downloadDir, err)
		}
	}
	results, results2, err := reseed.GetReseedTorrents(config.Get().ReseedUsername, config.Get().ReseedPassword,
		config.Get().Sites, timeout, savePathes...)
	if err != nil {
		return fmt.Errorf("failed to get xseed torrents from reseed server: %v", err)
	}
	if !doDownload {
		fmt.Fprintf(os.Stderr, "// full match (success) xseed torrents found by Reseed API\n")
		fmt.Println(strings.Join(results, "  "))
		fmt.Fprintf(os.Stderr, "// partial match (warning) xseed torrents found by Reseed API\n")
		fmt.Fprintln(os.Stderr, strings.Join(results2, "  "))
		fmt.Fprintf(os.Stderr, "// To download found torrents to local, run this command with --download flag,\n")
		fmt.Fprintf(os.Stderr, "// only full match torrents will be downloaded, unless --download-all flag is set.\n")
		return nil
	}

	var torrents []string
	torrents = append(torrents, results...)
	if downloadAll {
		torrents = append(torrents, results2...)
	}
	cntAll := len(torrents)
	cntSuccess := int64(0)
	cntSkip := int64(0)
	errorCnt := int64(0)
	for i, torrent := range torrents {
		fileName := torrent + ".torrent"
		if util.FileExists(filepath.Join(downloadDir, fileName)) ||
			util.FileExists(filepath.Join(downloadDir, fileName, ".added")) {
			fmt.Printf("! %s (%d/%d): already exists in %s , skip it.\n", torrent, i+1, cntAll, downloadDir)
			cntSkip++
			continue
		}
		content, tinfo, _, sitename, _, _, err := helper.GetTorrentContent(torrent, "", false, true, nil, true)
		if err != nil {
			fmt.Printf("✕ download %s (%d/%d): %v\n", torrent, i+1, cntAll, err)
			errorCnt++
			continue
		}
		if useComment {
			var useCommentErr error
			var tags []string
			tags = append(tags, config.XSEED_TAG)
			if sitename != "" {
				tags = append(tags, client.GenerateTorrentTagFromSite(sitename))
			}
			if err := tinfo.EncodeComment(&torrentutil.TorrentCommentMeta{
				Tags:     tags,
				SavePath: savePathes[0],
			}); err != nil {
				useCommentErr = fmt.Errorf("failed to encode: %v", err)
			} else if data, err := tinfo.ToBytes(); err != nil {
				useCommentErr = fmt.Errorf("failed to re-generate torrent: %v", err)
			} else {
				content = data
			}
			if useCommentErr != nil {
				fmt.Printf("✕ %s (%d/%d): failed to update comment: %v\n", torrent, i+1, cntAll, err)
				errorCnt++
				continue
			}
		}
		err = os.WriteFile(filepath.Join(downloadDir, fileName), content, 0666)
		if err != nil {
			fmt.Printf("✕ %s (%d/%d): failed to save to %s : %v\n", torrent, i+1, cntAll, downloadDir, err)
			errorCnt++
		} else {
			cntSuccess++
			fmt.Printf("✓ %s (%d/%d): saved to %s\n", torrent, i+1, cntAll, downloadDir)
		}
	}
	fmt.Printf("\n")
	if cntSuccess > 0 || cntSkip > 0 {
		fmt.Printf(`Saved %d torrents to %s
Error torrents (failed to download): %d
Skipped torrents (already exists in local): %d

To add downloaded torrents to local client as xseed torrents, run following command:
    ptool xseedadd <client> "%s/*.torrent"
`, cntSuccess, downloadDir, errorCnt, cntSkip, downloadDir)
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
