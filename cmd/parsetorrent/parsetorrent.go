package parsetorrent

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

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

To output parsed info in json format, use "--json" flag.

You can customize the output format of each torrent using "--format string" flag.
The data passed to the template is the parsed torrent info object,
which includes all fields of "TorrentMeta" type:

// https://github.com/sagan/ptool/blob/master/util/torrentutil/torrent.go
type TorrentMeta struct {
	InfoHash          string
	PiecesHash        string // sha1(torrent.info.pieces)
	Trackers          []string
	Size              int64
	SingleFileTorrent bool
	RootDir           string
	ContentPath       string // root folder or single file name
	Files             []TorrentMetaFile
	MetaInfo          *metainfo.MetaInfo // always non-nil in a parsed *TorrentMeta
	Info              *metainfo.Info     // always non-nil in a parsed *TorrentMeta
}

Some additionally fields are also available:
- Index : number. current torrent index
- Torrent : string. current torrent filename.

The template render result will be trim spaced.
If the renderring throws any error, the torrent will be treated as fail.

E.g. '--format "{{.Torrent}}: {{.InfoHash}} - {{.Size}}"'.

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
	matchTracker      = ""
	filter            = ""
	format            = ""
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
	command.Flags().StringVarP(&filter, "filter", "", "",
		"If set, treat torrent which name or content file names do not contain this value as fail (error)")
	command.Flags().StringVarP(&defaultSite, "site", "", "", "Set default site of torrent url")
	command.Flags().StringVarP(&minTorrentSizeStr, "min-torrent-size", "", "-1",
		"Treat torrent which contents size is smaller than (<) this value as fail (error). -1 == no limit")
	command.Flags().StringVarP(&maxTorrentSizeStr, "max-torrent-size", "", "-1",
		"Treat torrent which contents size is larger than (>) this value as fail (error). -1 == no limit")
	command.Flags().StringVarP(&matchTracker, "match-tracker", "", "",
		"Treat torrent which trackers does not contain this tracker (domain or url) as fail (error). "+
			`If set to "`+constants.NONE+`", it matches if torrent does NOT have any tracker`)
	command.Flags().StringVarP(&format, "format", "", "", `Manually set the output format of parsed torrent info. `+
		`Available variable placeholders: {{.InfoHash}}, {{.PiecesHash}}, {{.Size}} and more. `+
		constants.HELP_ARG_TEMPLATE+`. If renderring throws any error, the torrent will be treated as fail`)
	cmd.RootCmd.AddCommand(command)
}

func parsetorrent(cmd *cobra.Command, args []string) error {
	if cnt := util.CountNonZeroVariables(format, showInfoHashOnly, showAll, showSum, showJson); cnt > 1 {
		if cnt > 2 || !(showSum && showJson) {
			return fmt.Errorf("--format, --all, --show-info-hash-only, --json and --sum flags " +
				"are NOT compatible (except the last two)")
		}
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
		return fmt.Errorf("invalid min-torrent-size: %w", err)
	}
	maxTorrentSize, err := util.RAMInBytes(maxTorrentSizeStr)
	if err != nil {
		return fmt.Errorf("invalid max-torrent-size: %w", err)
	}
	errorCnt := int64(0)
	parsedTorrents := map[string]struct{}{}
	statistics := common.NewTorrentsStatistics()
	var outputTemplate *template.Template
	if format != "" {
		if outputTemplate, err = helper.GetTemplate(format); err != nil {
			return fmt.Errorf("invalid format template: %v", err)
		}
	}

	for i, torrent := range torrents {
		_, tinfo, _, _, _, _, isLocal, err := helper.GetTorrentContent(torrent, defaultSite, forceLocal, false,
			stdinTorrentContents, false, nil)
		customOutput := ""
		if err != nil {
			statistics.UpdateTinfo(common.TORRENT_INVALID, nil)
		} else {
			_, exists := parsedTorrents[tinfo.InfoHash]
			if minTorrentSize >= 0 && tinfo.Size < minTorrentSize {
				err = fmt.Errorf("torrent is too small: %s (%d)", util.BytesSize(float64(tinfo.Size)), tinfo.Size)
			} else if maxTorrentSize >= 0 && tinfo.Size > maxTorrentSize {
				err = fmt.Errorf("torrent is too large: %s (%d)", util.BytesSize(float64(tinfo.Size)), tinfo.Size)
			} else if dedupe && exists {
				err = fmt.Errorf("torrent is duplicate: info-hash = %s", tinfo.InfoHash)
			} else if matchTracker != "" && !tinfo.MatchTracker(matchTracker) {
				err = fmt.Errorf("torrent tracker(s) does not match: %v", tinfo.Trackers)
			} else if filter != "" && !tinfo.MatchFilter(filter) {
				err = fmt.Errorf("torrent name & content file names do not contain %q", filter)
			}
			if err != nil {
				statistics.UpdateTinfo(common.TORRENT_FAILURE, tinfo)
			} else if outputTemplate != nil {
				buf := &bytes.Buffer{}
				data := util.StructToMap(*tinfo, false, false)
				data["Index"] = i
				data["Torrent"] = torrent
				if err = outputTemplate.Execute(buf, data); err != nil {
					err = fmt.Errorf("failed to render torrent %v: %v", tinfo, err)
				} else {
					customOutput = strings.TrimSpace(buf.String())
				}
			}
		}
		if err != nil {
			if !showSum {
				fmt.Fprintf(os.Stderr, "✕ %s : %v\n", torrent, err)
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
		statistics.UpdateTinfo(common.TORRENT_SUCCESS, tinfo)
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
		} else if outputTemplate != nil {
			fmt.Println(customOutput)
			continue
		}
		tinfo.Fprint(os.Stdout, torrent, showAll)
		if showAll {
			tinfo.FprintFiles(os.Stdout, true, false)
			fmt.Printf("\n")
		}
	}
	if !showInfoHashOnly && outputTemplate == nil {
		if !showJson {
			fmt.Printf("\n")
			statistics.Print(os.Stdout)
		} else if showSum {
			if err := util.PrintJson(os.Stdout, statistics); err != nil {
				return err
			}
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
