package show

import (
	"fmt"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/utils"
)

var command = &cobra.Command{
	Use:   "show <client> [<infoHash>...]",
	Short: "Show torrents of client",
	Long: `Show torrents of client
<infoHash>...: infoHash list of torrents. It's possible to use state filter to target multiple torrents:
_all, _active, _done,  _downloading, _seeding, _paused, _completed, _error
If no infoHash are provided, it will display current active torrents
`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run:  show,
}

var (
	desc        = false
	maxTorrents = int64(0)
	filter      = ""
	category    = ""
	tag         = ""
	sortField   = ""
)

func init() {
	command.Flags().Int64VarP(&maxTorrents, "max-results", "m", 0, "Show at most this number of torrents. Default (0) == unlimited")
	command.Flags().BoolVarP(&desc, "desc", "d", false, "Used with --sort. Sort by desc order instead of asc")
	command.Flags().StringVarP(&filter, "filter", "f", "", "filter torrents by name")
	command.Flags().StringVarP(&category, "category", "c", "", "filter torrents by category")
	command.Flags().StringVarP(&tag, "tag", "t", "", "filter torrents by tag")
	command.Flags().StringVarP(&sortField, "sort", "s", "", "Sort torrents by this. Possible values: name|size|speed")
	cmd.RootCmd.AddCommand(command)
}

func show(cmd *cobra.Command, args []string) {
	clientInstance, err := client.CreateClient(args[0])
	if err != nil {
		log.Fatal(err)
	}
	args = args[1:]
	torrents := []client.Torrent{}

	if len(args) == 0 {
		_torrents, err := clientInstance.GetTorrents("", category, false)
		if err != nil {
			log.Errorf("Failed to fetch client torrents: %v", err)
		} else {
			torrents = append(torrents, _torrents...)
		}
	} else if slices.Index(args, "_all") != -1 {
		_torrents, err := clientInstance.GetTorrents("", category, true)
		if err != nil {
			log.Errorf("Failed to fetch client torrents: %v", err)
		} else {
			torrents = append(torrents, _torrents...)
		}
	} else {
		for _, arg := range args {
			if strings.HasPrefix(arg, "_") {
				_torrents, err := clientInstance.GetTorrents(arg, category, true)
				if err != nil {
					log.Errorf("Failed to fetch client torrents: %v", err)
				} else {
					torrents = append(torrents, _torrents...)
				}
			} else {
				torrent, err := clientInstance.GetTorrent(arg)
				if err == nil && torrent != nil {
					torrents = append(torrents, *torrent)
				}
			}
		}
	}

	torrents = utils.UniqueSliceFn(torrents, func(t client.Torrent) string {
		return t.InfoHash
	})
	if filter != "" || tag != "" {
		torrents = utils.Filter(torrents, func(t client.Torrent) bool {
			if filter != "" && !utils.ContainsI(t.Name, filter) {
				return false
			}
			if tag != "" && !t.HasTag(tag) {
				return false
			}
			return true
		})
	}
	if sortField != "" {
		if desc {
			sort.Slice(torrents, func(i, j int) bool {
				switch sortField {
				case "name":
					return torrents[i].Name > torrents[j].Name
				case "size":
					return torrents[i].Size > torrents[j].Size
				case "speed":
					return torrents[i].DownloadSpeed+torrents[i].UploadSpeed >
						torrents[j].DownloadSpeed+torrents[j].UploadSpeed
				}
				return i < j
			})
		} else {
			sort.Slice(torrents, func(i, j int) bool {
				switch sortField {
				case "name":
					return torrents[i].Name < torrents[j].Name
				case "size":
					return torrents[i].Size < torrents[j].Size
				case "speed":
					return torrents[i].DownloadSpeed+torrents[i].UploadSpeed <
						torrents[j].DownloadSpeed+torrents[j].UploadSpeed
				}
				return i < j
			})
		}
	}
	if maxTorrents > 0 && len(torrents) > int(maxTorrents) {
		torrents = torrents[:maxTorrents]
	}

	fmt.Printf("Client %s / Showing %d torrents\n\n", clientInstance.GetName(), len(torrents))
	client.PrintTorrents(torrents, "")
}
