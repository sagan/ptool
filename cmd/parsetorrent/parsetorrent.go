package parsetorrent

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/torrentutil"
)

var command = &cobra.Command{
	Use:         "parsetorrent {file.torrent}...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "parsetorrent"},
	Aliases:     []string{"parse"},
	Short:       "Parse torrent files and show their content.",
	Long:        `Parse torrent files and show their content.`,
	Args:        cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE:        parsetorrent,
}

var (
	showAll  = false
	showJson = false
)

func init() {
	command.Flags().BoolVarP(&showAll, "all", "a", false, "Show all info")
	command.Flags().BoolVarP(&showJson, "json", "", false, "Show output in json format")
	cmd.RootCmd.AddCommand(command)
}

func parsetorrent(cmd *cobra.Command, args []string) error {
	torrentFilenames := util.ParseFilenameArgs(args...)
	errorCnt := int64(0)

	for i, torrentFilename := range torrentFilenames {
		var torrentContent []byte
		var err error
		if torrentFilename == "-" {
			torrentContent, err = io.ReadAll(os.Stdin)
		} else {
			torrentContent, err = os.ReadFile(torrentFilename)
		}
		if err != nil {
			log.Errorf("Failed to read %s: %v", torrentFilename, err)
			errorCnt++
			continue
		}
		torrentInfo, err := torrentutil.ParseTorrent(torrentContent, 99)
		if err != nil {
			log.Errorf("Failed to parse %s: %v", torrentFilename, err)
			errorCnt++
			continue
		}
		if showJson {
			bytes, err := json.Marshal(torrentInfo)
			if err != nil {
				log.Errorf("Failed to marshal info json of %s: %v", torrentFilename, err)
				errorCnt++
				continue
			}
			fmt.Println(string(bytes))
			continue
		}
		torrentInfo.Print(torrentFilename, showAll)
		if showAll {
			torrentInfo.PrintFiles(true, false)
			if i < len(torrentFilenames)-1 {
				fmt.Printf("\n")
			}
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
