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
	Aliases:     []string{"parse"},
	Short:       "Parse torrent files and show their contents.",
	Long: fmt.Sprintf(`Parse torrent files and show their contents.
%s.`, constants.HELP_TORRENT_ARGS),
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: parsetorrent,
}

var (
	showAll          = false
	showInfoHashOnly = false
	showJson         = false
	forceLocal       = false
	showSum          = false
	renameFail       = false
	defaultSite      = ""
)

func init() {
	command.Flags().BoolVarP(&renameFail, "rename-fail", "", false,
		"Rename invalid (failed to parse) torrent file to *"+constants.FILENAME_SUFFIX_FAIL)
	command.Flags().BoolVarP(&showAll, "all", "a", false, "Show all info")
	command.Flags().BoolVarP(&showInfoHashOnly, "show-info-hash-only", "", false, "Output torrents info hash only")
	command.Flags().BoolVarP(&showJson, "json", "", false, "Show output in json format")
	command.Flags().BoolVarP(&forceLocal, "force-local", "", false, "Force treat all arg as local torrent filename")
	command.Flags().BoolVarP(&showSum, "sum", "", false, "Show torrents summary only")
	command.Flags().StringVarP(&defaultSite, "site", "", "", "Set default site of torrent url")
	cmd.RootCmd.AddCommand(command)
}

func parsetorrent(cmd *cobra.Command, args []string) error {
	if util.CountNonZeroVariables(showInfoHashOnly, showAll, showSum, showJson) > 1 {
		return fmt.Errorf("--all, --show-info-hash-only, --sum and --json flags are NOT compatible")
	}
	torrents, stdinTorrentContents, err := helper.ParseTorrentsFromArgs(args)
	if err != nil {
		return err
	}
	errorCnt := int64(0)
	allSize := int64(0)
	cntTorrents := int64(0)

	for _, torrent := range torrents {
		_, tinfo, _, _, _, _, isLocal, err := helper.GetTorrentContent(torrent, defaultSite, forceLocal, false,
			stdinTorrentContents, false, nil)
		if err != nil {
			log.Errorf("Failed to parse %s: %v", torrent, err)
			errorCnt++
			if isLocal && torrent != "-" && renameFail &&
				!strings.HasSuffix(torrent, constants.FILENAME_SUFFIX_FAIL) && util.FileExists(torrent) {
				if err := os.Rename(torrent, util.TrimAnySuffix(torrent,
					constants.ProcessedFilenameSuffixes...)+constants.FILENAME_SUFFIX_FAIL); err != nil {
					log.Debugf("Failed to rename %s to *%s: %v", torrent, constants.FILENAME_SUFFIX_FAIL, err)
				}
			}
			continue
		}
		cntTorrents++
		allSize += tinfo.Size
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
		tinfo.Print(torrent, showAll)
		if showAll {
			tinfo.PrintFiles(true, false)
			fmt.Printf("\n")
		}
	}
	sumOutputDst := os.Stderr
	if showSum {
		sumOutputDst = os.Stdout
	}
	fmt.Fprintf(sumOutputDst, "\n")
	fmt.Fprintf(sumOutputDst, "// Total torrents: %d\n", cntTorrents)
	fmt.Fprintf(sumOutputDst, "// Total size: %s (%d Byte)\n", util.BytesSize(float64(allSize)), allSize)
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
