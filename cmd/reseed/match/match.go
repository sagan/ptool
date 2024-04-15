package match

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd/reseed"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
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
By default only full match (success) torrents will be included,
use --all flag to include partial-match (warning) results.

To download found torrents to local, use --download flag,

By default it downloads torrents to "<config_dir>/reseed" dir, where the <config_dir>
is the folder that ptool.toml config file is located at. Use --download-dir to change it.

Existing torrents in local disk (already downloaded before) will be skipped (do NOT re-download it).

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
	slowMode           = false
	showJson           = false
	showRaw            = false
	useComment         = false
	doDownload         = false
	all                = false
	timeout            = int64(0)
	maxConsecutiveFail = int64(0)
	downloadDir        = ""
)

func init() {
	command.Flags().BoolVarP(&slowMode, "slow", "", false, "Slow mode. wait after downloading each xseed torrent")
	command.Flags().BoolVarP(&showJson, "json", "", false, "Show full info in json format for found torrents")
	command.Flags().BoolVarP(&showRaw, "raw", "", false, "Show raw Reseed torrent id for found torrents")
	command.Flags().BoolVarP(&useComment, "use-comment-meta", "", false,
		"Use with --download. Use 'comment' field to export save path location to downloaded .torrent files")
	command.Flags().BoolVarP(&doDownload, "download", "", false, "Download found xseed torrents to local")
	command.Flags().BoolVarP(&all, "all", "", false,
		"Display or download all found xseed torrents (include partial-match results)")
	command.Flags().Int64VarP(&timeout, "timeout", "", 15, "Timeout (seconds) for requesting Reseed API")
	command.Flags().Int64VarP(&maxConsecutiveFail, "max-consecutive-fail", "", 3,
		"After consecutive fails to download torrent from a site of this times, will skip that site afterwards. "+
			"Note a 404 error does NOT count as a fail. -1 = no limit (never skip)")
	command.Flags().StringVarP(&downloadDir, "download-dir", "", "",
		`Set the dir of downloaded .torrent files. By default it uses "<config_dir>/reseed"`)
	reseed.Command.AddCommand(command)
}

func match(cmd *cobra.Command, args []string) error {
	savePathes := args
	if config.Get().ReseedUsername == "" || config.Get().ReseedPassword == "" {
		return fmt.Errorf("you must config reseedUsername & reseedPassword in ptool.toml to use reseed functions")
	}
	if util.CountNonZeroVariables(showJson, showRaw, doDownload) > 1 {
		return fmt.Errorf("--json & --raw & --download flags are NOT compatible")
	}
	if timeout <= 0 {
		return fmt.Errorf("timeout must be > 0")
	}
	if downloadDir == "" {
		downloadDir = filepath.Join(config.ConfigDir, "reseed")
	}
	if doDownload {
		if err := os.MkdirAll(downloadDir, constants.PERM); err != nil {
			return fmt.Errorf("failed to create download-dir %s: %v", downloadDir, err)
		}
	}
	results, results2, err := reseed.GetReseedTorrents(config.Get().ReseedUsername, config.Get().ReseedPassword,
		config.Get().Sites, timeout, savePathes...)
	if err != nil {
		return fmt.Errorf("failed to get xseed torrents from reseed server: %v", err)
	}
	var torrents []*reseed.Torrent
	torrents = append(torrents, results...)
	if all {
		torrents = append(torrents, results2...)
	}
	if showJson {
		if bytes, err := json.Marshal(torrents); err != nil {
			return fmt.Errorf("failed to marshal json: %v", err)
		} else {
			fmt.Println(string(bytes))
			return nil
		}
	} else if showRaw {
		fmt.Println(strings.Join(util.Map(torrents, func(t *reseed.Torrent) string { return t.ReseedId }), "  "))
		return nil
	} else if !doDownload {
		if all {
			fmt.Fprintf(os.Stderr, "// All xseed (include partial match (warning)) torrents found by Reseed API\n")
		} else {
			fmt.Fprintf(os.Stderr, "// Full match (success) xseed torrents found by Reseed API\n")
		}
		fmt.Println(strings.Join(util.MapString(torrents), "  "))
		fmt.Fprintf(os.Stderr, "// To download these torrents to local, run this command with --download flag,\n")
		return nil
	}

	cntAll := len(torrents)
	cntSuccess := int64(0)
	cntSkip := int64(0)
	errorCnt := int64(0)
	siteConsecutiveFails := map[string]int64{}
	for i, torrent := range torrents {
		if torrent.Id == "" {
			log.Debugf("! ignore reseed torrent %s which site does NOT exists in local", torrent.ReseedId)
			cntSkip++
			continue
		}
		filename := torrent.Id + ".torrent"
		if util.FileExistsWithOptionalSuffix(filepath.Join(downloadDir, filename), constants.ProcessedFilenameSuffixes...) {
			log.Debugf("! %s (%d/%d): already exists in %s , skip it.\n", torrent, i+1, cntAll, downloadDir)
			cntSkip++
			continue
		}
		sitename, _, found := strings.Cut(torrent.Id, ".")
		if found && sitename != "" && maxConsecutiveFail >= 0 && siteConsecutiveFails[sitename] > maxConsecutiveFail {
			log.Debugf("Skip site %s torrent %s as this site has failed too much times", sitename, torrent.Id)
			continue
		}
		if i > 0 && slowMode {
			util.Sleep(3)
		}
		content, tinfo, _, sitename, _, _, _, err := helper.GetTorrentContent(torrent.Id, "", false, true, nil, true, nil)
		if err != nil {
			fmt.Printf("✕ download %s (%d/%d): %v\n", torrent, i+1, cntAll, err)
			errorCnt++
			if sitename != "" {
				if !strings.Contains(err.Error(), "status=404") {
					siteConsecutiveFails[sitename]++
					if maxConsecutiveFail >= 0 && siteConsecutiveFails[sitename] == maxConsecutiveFail {
						log.Errorf("Site %s has failed (to download torrent) too many times, skip it from now", sitename)
					}
				} else {
					siteConsecutiveFails[sitename] = 0
				}
			}
			continue
		}
		if sitename != "" {
			siteConsecutiveFails[sitename] = 0
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
				SavePath: torrent.SavePath,
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
		err = os.WriteFile(filepath.Join(downloadDir, filename), content, constants.PERM)
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
Skipped torrents (local site does NOT exist, or already downloaded before): %d

To add downloaded torrents to local client as xseed torrents, run following command:
    ptool xseedadd <client> "%s/*.torrent"
`, cntSuccess, downloadDir, errorCnt, cntSkip, downloadDir)
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
