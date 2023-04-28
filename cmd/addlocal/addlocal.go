package add

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
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
	errCnt := int64(0)
	torrentFiles := args[1:]
	option := &client.TorrentOption{
		Pause:    paused,
		Category: setCategory,
	}
	if addTags != "" {
		option.Tags = strings.Split(addTags, ",")
	}

	for _, torrentFile := range torrentFiles {
		torrentContent, err := os.ReadFile(torrentFile)
		if err != nil {
			fmt.Printf("torrent %s: failed to read file (%v)\n", torrentFile, err)
			errCnt++
			continue
		}
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
