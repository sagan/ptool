package parsetorrent

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/common"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use:         "parsetorrent {torrentFilename | torrentId | torrentUrl}...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "parsetorrent"},
	Aliases:     []string{"parse", "parsetorrents"},
	Short:       "Parse .torrent (metainfo) files and show their contents.",
	Long: fmt.Sprintf(`Parse .torrent (metainfo) files and show their contents.
%s.

By default it displays parsed infos of all provided torrents.
If "--sum" flag is set, it only displays the summary of all torrents.

It's also capable to work as a torrent files "filter", e.g. :
  ptool parsetorrent --dedupe --max-torrent-size 100MiB --rename-fail --sum *.torrent
It will treat all torrents which is duplicate (has the same info-hash as a previous torrent)
or which contents size is larger than 100MiB as fail (error),
and rename these torrent files to *%s suffix.`,
		constants.HELP_TORRENT_ARGS, constants.FILENAME_SUFFIX_FAIL),
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: parsetorrent,
}

var (
	dedupe            = false
	showAll           = false
	showInfoHashOnly  = false
	showJson          = false
	forceLocal        = false
	showSum           = false
	renameFail        = false
	deleteFail        = false
	defaultSite       = ""
	minTorrentSizeStr = ""
	maxTorrentSizeStr = ""
)

func init() {
	command.Flags().BoolVarP(&renameFail, "rename-fail", "", false,
		"Rename fail (failed to parse, or treated as error) .torrent file to *"+constants.FILENAME_SUFFIX_FAIL+
			` unless it's name already has that suffix. It will only rename file which has `+
			`".torrent" or ".torrent.*" extension`)
	command.Flags().BoolVarP(&deleteFail, "delete-fail", "", false,
		`Delete fail (failed to parse, or treated as error) .torrent file. `+
			`It will only delete file which has ".torrent" or ".torrent.*" extension`)
	command.Flags().BoolVarP(&dedupe, "dedupe", "", false,
		"Treat duplicate torrent (has the same info-hash as previous parsed torrent) as fail (error)")
	command.Flags().BoolVarP(&showAll, "all", "a", false, "Show all info")
	command.Flags().BoolVarP(&showInfoHashOnly, "show-info-hash-only", "", false, "Output torrents info hash only")
	command.Flags().BoolVarP(&showJson, "json", "", false, "Show output in json format")
	command.Flags().BoolVarP(&forceLocal, "force-local", "", false, "Force treat all arg as local torrent filename")
	command.Flags().BoolVarP(&showSum, "sum", "", false, "Show torrents summary only")
	command.Flags().StringVarP(&defaultSite, "site", "", "", "Set default site of torrent url")
	command.Flags().StringVarP(&minTorrentSizeStr, "min-torrent-size", "", "-1",
		"Treat torrent which contents size is smaller than (<) this value as fail (error). -1 == no limit")
	command.Flags().StringVarP(&maxTorrentSizeStr, "max-torrent-size", "", "-1",
		"Treat torrent which contents size is larger than (>) this value as fail (error). -1 == no limit")
	cmd.RootCmd.AddCommand(command)
}

func parsetorrent(cmd *cobra.Command, args []string) error {
	if util.CountNonZeroVariables(showInfoHashOnly, showAll, showSum) > 1 {
		return fmt.Errorf("--all, --show-info-hash-only and --sum flags are NOT compatible")
	}
	if util.CountNonZeroVariables(showInfoHashOnly, showAll, showJson) > 1 {
		return fmt.Errorf("--all, --show-info-hash-only and --json flags are NOT compatible")
	}
	if renameFail && deleteFail {
		return fmt.Errorf("--rename-fail and --delete-fail flags are NOT compatible")
	}
	torrents, stdinTorrentContents, err := helper.ParseTorrentsFromArgs(args)
	if err != nil {
		return err
	}
	minTorrentSize, err := util.RAMInBytes(minTorrentSizeStr)
	if err != nil {
		return fmt.Errorf("invalid min-torrent-size: %v", err)
	}
	maxTorrentSize, err := util.RAMInBytes(maxTorrentSizeStr)
	if err != nil {
		return fmt.Errorf("invalid max-torrent-size: %v", err)
	}
	errorCnt := int64(0)
	parsedTorrents := map[string]struct{}{}
	statistics := common.NewTorrentsStatistics()

	for _, torrent := range torrents {
		_, tinfo, _, _, _, _, isLocal, err := helper.GetTorrentContent(torrent, defaultSite, forceLocal, false,
			stdinTorrentContents, false, nil)
		if err != nil {
			statistics.Update(common.TORRENT_INVALID, nil)
		} else {
			_, exists := parsedTorrents[tinfo.InfoHash]
			if minTorrentSize >= 0 && tinfo.Size < minTorrentSize {
				err = fmt.Errorf("torrent is too small: %s (%d)", util.BytesSize(float64(tinfo.Size)), tinfo.Size)
			} else if maxTorrentSize >= 0 && tinfo.Size > maxTorrentSize {
				err = fmt.Errorf("torrent is too large: %s (%d)", util.BytesSize(float64(tinfo.Size)), tinfo.Size)
			} else if dedupe && exists {
				err = fmt.Errorf("torrent is duplicate: info-hash = %s", tinfo.InfoHash)
			}
			if err != nil {
				statistics.Update(common.TORRENT_FAILURE, tinfo)
			}
		}
		if err != nil {
			if !showSum {
				fmt.Fprintf(os.Stderr, "✕ %s : failed to parse: %v\n", torrent, err)
			}
			errorCnt++
			if isLocal && torrent != "-" {
				torrentTrim := util.TrimAnySuffix(torrent, constants.ProcessedFilenameSuffixes...)
				isValidTarget := strings.HasSuffix(torrentTrim, ".torrent") && util.FileExists(torrent)
				if renameFail && !strings.HasSuffix(torrent, constants.FILENAME_SUFFIX_FAIL) && isValidTarget {
					if err := os.Rename(torrent, torrentTrim+constants.FILENAME_SUFFIX_FAIL); err != nil {
						log.Debugf("Failed to rename %s to *%s: %v", torrent, constants.FILENAME_SUFFIX_FAIL, err)
					}
				} else if deleteFail && isValidTarget {
					if err := os.Remove(torrent); err != nil {
						log.Debugf("Failed to delete %s: %v", torrent, err)
					}
				}
			}
			continue
		}
		parsedTorrents[tinfo.InfoHash] = struct{}{}
		statistics.Update(common.TORRENT_SUCCESS, tinfo)
		if showSum {
			continue
		}
		if showJson {
			if err := util.PrintJson(os.Stdout, tinfo); err != nil {
				log.Errorf("%s: %v", torrent, err)
				errorCnt++
			}
			continue
		} else if showInfoHashOnly {
			fmt.Printf("%s\n", tinfo.InfoHash)
			continue
		}
		tinfo.Fprint(os.Stdout, torrent, showAll)
		if showAll {
			tinfo.FprintFiles(os.Stdout, true, false)
			fmt.Printf("\n")
		}
	}
	if !showInfoHashOnly {
		if !showJson {
			fmt.Printf("\n")
			statistics.Print(os.Stdout)
		} else if err := util.PrintJson(os.Stdout, statistics); err != nil {
			return err
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
