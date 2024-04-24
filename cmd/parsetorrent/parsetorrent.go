package parsetorrent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
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
			" unless it's name already has that suffix")
	command.Flags().BoolVarP(&deleteFail, "delete-fail", "", false,
		`Delete fail (failed to parse, or treated as error) .torrent file. `+
			`It will only delete file which has ".torrent" extension`)
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
	if util.CountNonZeroVariables(showInfoHashOnly, showAll, showSum, showJson) > 1 {
		return fmt.Errorf("--all, --show-info-hash-only, --sum and --json flags are NOT compatible")
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
	allSize := int64(0)
	cntTorrents := int64(0)
	cntFiles := int64(0)
	parsedTorrents := map[string]struct{}{}
	smallestSize := int64(-1)
	largestSize := int64(-1)

	for _, torrent := range torrents {
		_, tinfo, _, _, _, _, isLocal, err := helper.GetTorrentContent(torrent, defaultSite, forceLocal, false,
			stdinTorrentContents, false, nil)
		if err == nil {
			_, exists := parsedTorrents[tinfo.InfoHash]
			if minTorrentSize >= 0 && tinfo.Size < minTorrentSize {
				err = fmt.Errorf("torrent is too small: %s (%d)", util.BytesSize(float64(tinfo.Size)), tinfo.Size)
			} else if maxTorrentSize >= 0 && tinfo.Size > maxTorrentSize {
				err = fmt.Errorf("torrent is too large: %s (%d)", util.BytesSize(float64(tinfo.Size)), tinfo.Size)
			} else if dedupe && exists {
				err = fmt.Errorf("torrent is duplicate: info-hash = %s", tinfo.InfoHash)
			}
		}
		if err != nil {
			log.Errorf("Failed to parse %s: %v", torrent, err)
			errorCnt++
			if isLocal && torrent != "-" {
				if renameFail && !strings.HasSuffix(torrent, constants.FILENAME_SUFFIX_FAIL) && util.FileExists(torrent) {
					if err := os.Rename(torrent, util.TrimAnySuffix(torrent,
						constants.ProcessedFilenameSuffixes...)+constants.FILENAME_SUFFIX_FAIL); err != nil {
						log.Debugf("Failed to rename %s to *%s: %v", torrent, constants.FILENAME_SUFFIX_FAIL, err)
					}
				} else if deleteFail && strings.HasSuffix(torrent, ".torrent") && util.FileExists(torrent) {
					if err := os.Remove(torrent); err != nil {
						log.Debugf("Failed to delete %s: %v", torrent, err)
					}
				}
			}
			continue
		}
		parsedTorrents[tinfo.InfoHash] = struct{}{}
		cntTorrents++
		allSize += tinfo.Size
		cntFiles += int64(len(tinfo.Files))
		if largestSize == -1 || tinfo.Size > largestSize {
			largestSize = tinfo.Size
		}
		if smallestSize == -1 || tinfo.Size < smallestSize {
			smallestSize = tinfo.Size
		}
		if showSum {
			continue
		}
		if showJson {
			bytes, err := json.Marshal(tinfo)
			if err != nil {
				log.Errorf("Failed to marshal info json of %s: %v", torrent, err)
				errorCnt++
				continue
			}
			fmt.Println(string(bytes))
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
	sumOutputDst := os.Stderr
	if showSum {
		sumOutputDst = os.Stdout
	}
	averageSize := int64(0)
	if cntTorrents > 0 {
		averageSize = allSize / cntTorrents
	}
	fmt.Fprintf(sumOutputDst, "\n")
	fmt.Fprintf(sumOutputDst, "// Total torrents: %d\n", cntTorrents)
	fmt.Fprintf(sumOutputDst, "// Total contents size: %s (%d Byte)\n", util.BytesSize(float64(allSize)), allSize)
	fmt.Fprintf(sumOutputDst, "// Total number of content files in torrents: %d\n", cntFiles)
	fmt.Fprintf(sumOutputDst, "// Smallest / Average / Largest torrent contents size: %s / %s / %s\n",
		util.BytesSize(float64(smallestSize)), util.BytesSize(float64(averageSize)), util.BytesSize(float64(largestSize)))
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
