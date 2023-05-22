package add

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	goTorrentParser "github.com/j-muller/go-torrent-parser"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/site/tpl"
	"github.com/sagan/ptool/utils"
)

var command = &cobra.Command{
	Use:   "addlocal <client> <filename.torrent>...",
	Short: "Add local torrents to client",
	Long:  `Add local torrents to client.`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	Run:   add,
}

var (
	paused      = false
	skipCheck   = false
	defaultSite = ""
	rename      = ""
	addCategory = ""
	addTags     = ""
	savePath    = ""
)

func init() {
	command.Flags().BoolVarP(&skipCheck, "skip-check", "", false, "Skip hash checking when adding torrents")
	command.Flags().BoolVarP(&paused, "paused", "p", false, "Add torrents to client in paused state")
	command.Flags().StringVarP(&savePath, "add-save-path", "", "", "Set save path of added torrents")
	command.Flags().StringVarP(&defaultSite, "site", "", "", "Set default site of torrents")
	command.Flags().StringVarP(&addCategory, "add-category", "", "", "Set category of added torrents")
	command.Flags().StringVarP(&rename, "rename", "", "", "Rename added torrent (for dev/test only)")
	command.Flags().StringVarP(&addTags, "add-tags", "", "", "Set tags of added torrent (comma-separated)")
	cmd.RootCmd.AddCommand(command)
}

func add(cmd *cobra.Command, args []string) {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}
	errCnt := int64(0)
	torrentFiles := utils.ParseFilenameArgs(args[1:]...)
	option := &client.TorrentOption{
		Pause:        paused,
		Category:     addCategory,
		SavePath:     savePath,
		SkipChecking: skipCheck,
		Name:         rename,
	}
	var fixedTags []string
	if addTags != "" {
		fixedTags = strings.Split(addTags, ",")
	}

	for _, torrentFile := range torrentFiles {
		torrentContent, err := os.ReadFile(torrentFile)
		if err != nil {
			fmt.Printf("torrent %s: failed to read file (%v)\n", torrentFile, err)
			errCnt++
			continue
		}

		tinfo, err := goTorrentParser.Parse(bytes.NewReader(torrentContent))
		if err != nil {
			fmt.Printf("torrent %s: failed to parse torrent (%v)\n", torrentFile, err)
			errCnt++
			continue
		}
		sitename := ""
		for _, tracker := range tinfo.Announce {
			domain := utils.GetUrlDomain(tracker)
			if domain == "" {
				continue
			}
			sitename = tpl.GuessSiteByDomain(domain, defaultSite)
			if sitename != "" {
				break
			}
		}
		option.Tags = []string{}
		if sitename != "" {
			option.Tags = append(option.Tags, client.GenerateTorrentTagFromSite(sitename))
		}
		option.Tags = append(option.Tags, fixedTags...)

		err = clientInstance.AddTorrent(torrentContent, option, nil)
		if err != nil {
			fmt.Printf("torrent %s: failed to add to client (%v)\n", torrentFile, err)
			errCnt++
			continue
		}
		fmt.Printf("torrent %s: added to client\n", torrentFile)
	}
	if errCnt > 0 {
		os.Exit(1)
	}
}
