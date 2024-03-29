package parsetorrent

import (
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use:         "parsetorrent {torrentFilename | torrentId | torrentUrl}...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "parsetorrent"},
	Aliases:     []string{"parse"},
	Short:       "Parse torrent files and show their content.",
	Long: `Parse torrent files and show their content.
Args is torrent list that each one could be a local filename (e.g. "*.torrent" or "[M-TEAM]CLANNAD.torrent"),
site torrent id (e.g.: "mteam.488424") or url (e.g.: "https://kp.m-team.cc/details.php?id=488424").
Torrent url that does NOT belong to any site (e.g.: a public site url) is also supported.
Use a single "-" to read .torrent file contents from stdin.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: parsetorrent,
}

var (
	showAll     = false
	showJson    = false
	forceLocal  = false
	defaultSite = ""
)

func init() {
	command.Flags().BoolVarP(&showAll, "all", "a", false, "Show all info")
	command.Flags().BoolVarP(&showJson, "json", "", false, "Show output in json format")
	command.Flags().BoolVarP(&forceLocal, "force-local", "", false, "Force treat all arg as local torrent filename")
	command.Flags().StringVarP(&defaultSite, "site", "", "", "Set default site of torrent url")
	cmd.RootCmd.AddCommand(command)
}

func parsetorrent(cmd *cobra.Command, args []string) error {
	torrents := util.ParseFilenameArgs(args...)
	errorCnt := int64(0)

	for i, torrent := range torrents {
		_, tinfo, _, _, _, _, _, err := helper.GetTorrentContent(torrent, defaultSite, forceLocal, false, nil, false)
		if err != nil {
			log.Errorf("Failed to get %s: %v", torrent, err)
			errorCnt++
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
		}
		tinfo.Print(torrent, showAll)
		if showAll {
			tinfo.PrintFiles(true, false)
			if i < len(torrents)-1 {
				fmt.Printf("\n")
			}
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
