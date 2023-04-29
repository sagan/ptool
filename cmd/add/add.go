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
	"github.com/sagan/ptool/site"
)

var command = &cobra.Command{
	Use:   "add <client> <site> <torrentIdOrUrl>...",
	Short: "Add site torrents to client",
	Long:  `Add site torrents to client.`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(3), cobra.OnlyValidArgs),
	Run:   add,
}

var (
	paused      = false
	setCategory = ""
	addTags     = ""
)

func init() {
	command.Flags().BoolVarP(&paused, "paused", "p", false, "Add torrents to client in paused state")
	command.Flags().StringVar(&setCategory, "set-category", "", "Set category of added torrents.")
	command.Flags().StringVar(&addTags, "add-tags", "", "Add tags to added torrent (comma-separated).")
	cmd.RootCmd.AddCommand(command)
}

func add(cmd *cobra.Command, args []string) {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}
	siteInstance, err := site.CreateSite(args[1])
	if err != nil {
		log.Fatal(err)
	}
	errCnt := int64(0)
	torrentIds := args[2:]
	option := &client.TorrentOption{
		Pause:    paused,
		Category: setCategory,
	}
	if addTags != "" {
		option.Tags = strings.Split(addTags, ",")
	}

	for _, torrentId := range torrentIds {
		torrentContent, _, err := siteInstance.DownloadTorrent(torrentId)
		if err != nil {
			fmt.Printf("add site %s torrent %s error: failed to get site torrent: %v\n", siteInstance.GetName(), torrentId, err)
			errCnt++
			continue
		}
		tinfo, err := goTorrentParser.Parse(bytes.NewReader(torrentContent))
		if err != nil {
			fmt.Printf("add site %s torrent %s error: failed to parse torrent: %v\n", siteInstance.GetName(), torrentId, err)
			errCnt++
			continue
		}
		err = clientInstance.AddTorrent(torrentContent, option, nil)
		if err != nil {
			fmt.Printf("add site %s torrent %s error: failed to add torrent to client: %v\n", siteInstance.GetName(), torrentId, err)
			errCnt++
			continue
		}
		fmt.Printf("add site %s torrent %s success. infoHash=%s\n", siteInstance.GetName(), torrentId, tinfo.InfoHash)
	}
	if errCnt > 0 {
		os.Exit(1)
	}
}
