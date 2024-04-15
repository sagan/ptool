package parsetorrent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use:         "parsetorrent {torrentFilename | torrentId | torrentUrl}...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "parsetorrent"},
	Aliases:     []string{"parse"},
	Short:       "Parse torrent files and show their content.",
	Long: `Parse torrent files and show their content.
Args is torrent list that each one could be
a local filename (e.g. "*.torrent" or "[M-TEAM]CLANNAD.torrent"),
site torrent id (e.g.: "mteam.488424") or url (e.g.: "https://kp.m-team.cc/details.php?id=488424").
Torrent url that does NOT belong to any site (e.g.: a public site url), as well as "magnet:" link, is also supported.
Use a single "-" as args to read torrent list from stdin, delimited by blanks,
as a special case, it also supports directly reading .torrent file contents from stdin.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: parsetorrent,
}

var (
	showAll          = false
	showInfoHashOnly = false
	showJson         = false
	forceLocal       = false
	showSum          = false
	defaultSite      = ""
)

func init() {
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
	// directly read a torrent content from stdin.
	stdinTorrentContents := []byte{}
	torrents := util.ParseFilenameArgs(args...)
	if len(torrents) == 1 && torrents[0] == "-" {
		if config.InShell {
			return fmt.Errorf(`"-" arg can not be used in shell`)
		}
		if stdin, err := io.ReadAll(os.Stdin); err != nil {
			return fmt.Errorf("failed to read stdin: %v", err)
		} else if bytes.HasPrefix(stdin, []byte(constants.TORRENT_FILE_MAGIC_NUMBER)) ||
			bytes.HasPrefix(stdin, []byte(constants.TORRENT_FILE_MAGIC_NUMBER2)) {
			stdinTorrentContents = stdin
		} else if data, err := shlex.Split(string(stdin)); err != nil {
			return fmt.Errorf("failed to parse stdin to tokens: %v", err)
		} else {
			torrents = data
		}
	}

	errorCnt := int64(0)
	allSize := int64(0)
	cntTorrents := int64(0)

	for _, torrent := range torrents {
		_, tinfo, _, _, _, _, _, err := helper.GetTorrentContent(torrent, defaultSite, forceLocal, false,
			stdinTorrentContents, false, nil)
		if err != nil {
			log.Errorf("Failed to get %s: %v", torrent, err)
			errorCnt++
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
	fmt.Printf("\n")
	fmt.Printf("// Total torrents: %d\n", cntTorrents)
	fmt.Printf("// Total size: %s (%d Byte)\n", util.BytesSize(float64(allSize)), allSize)
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
