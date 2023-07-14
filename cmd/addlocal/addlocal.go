package add

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site/tpl"
	"github.com/sagan/ptool/torrentutil"
	"github.com/sagan/ptool/utils"
)

var command = &cobra.Command{
	Use:   "addlocal <client> <filename.torrent>...",
	Short: "Add local torrents to client.",
	Long: `Add local torrents to client.
It's possible to use "*" wildcard in filename to match multiple torrents. eg. "*.torrent".
`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	Run:  add,
}

var (
	paused          = false
	skipCheck       = false
	renameAdded     = false
	deleteAdded     = false
	addCategoryAuto = false
	defaultSite     = ""
	rename          = ""
	addCategory     = ""
	addTags         = ""
	savePath        = ""
)

func init() {
	command.Flags().BoolVarP(&skipCheck, "skip-check", "", false, "Skip hash checking when adding torrents")
	command.Flags().BoolVarP(&renameAdded, "rename-added", "", false, "Rename successfully added torrents to .added extension")
	command.Flags().BoolVarP(&deleteAdded, "delete-added", "", false, "Delete successfully added torrents")
	command.Flags().BoolVarP(&paused, "add-paused", "", false, "Add torrents to client in paused state")
	command.Flags().BoolVarP(&addCategoryAuto, "add-category-auto", "", false, "Automatically set category of added torrent to corresponding sitename")
	command.Flags().StringVarP(&savePath, "add-save-path", "", "", "Set save path of added torrents")
	command.Flags().StringVarP(&defaultSite, "site", "", "", "Set default site of torrents")
	command.Flags().StringVarP(&addCategory, "add-category", "", "", "Manually set category of added torrents")
	command.Flags().StringVarP(&rename, "rename", "", "", "Rename added torrent (for dev/test only)")
	command.Flags().StringVarP(&addTags, "add-tags", "", "", "Set tags of added torrent (comma-separated)")
	cmd.RootCmd.AddCommand(command)
}

func add(cmd *cobra.Command, args []string) {
	clientName := args[0]
	args = args[1:]
	if renameAdded && deleteAdded {
		log.Fatalf("--rename-added and --delete-added flags are NOT compatible")
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		log.Fatal(err)
	}
	errCnt := int64(0)
	torrentFiles := utils.ParseFilenameArgs(args...)
	if rename != "" && len(torrentFiles) > 1 {
		log.Fatalf("--rename flag can only be used with exact one torrent file arg")
	}
	option := &client.TorrentOption{
		Pause:        paused,
		SavePath:     savePath,
		SkipChecking: skipCheck,
		Name:         rename,
	}
	var fixedTags []string
	if addTags != "" {
		fixedTags = strings.Split(addTags, ",")
	}
	cntAll := len(torrentFiles)
	cntAdded := int64(0)
	sizeAdded := int64(0)

	for i, torrentFile := range torrentFiles {
		if strings.HasSuffix(torrentFile, ".added") {
			log.Tracef("!torrent (%d/%d) %s: skipped", i+1, cntAll, torrentFile)
			continue
		}
		torrentContent, err := os.ReadFile(torrentFile)
		if err != nil {
			fmt.Printf("✕torrent (%d/%d) %s: failed to read file (%v)\n", i+1, cntAll, torrentFile, err)
			errCnt++
			continue
		}
		tinfo, err := torrentutil.ParseTorrent(torrentContent, 99)
		if err != nil {
			fmt.Printf("✕torrent (%d/%d) %s: failed to parse torrent (%v)\n", i+1, cntAll, torrentFile, err)
			errCnt++
			continue
		}
		sitename := tpl.GuessSiteByTrackers(tinfo.Trackers, defaultSite)
		if addCategoryAuto {
			if sitename != "" {
				option.Category = sitename
			} else if addCategory != "" {
				option.Category = addCategory
			} else {
				option.Category = "Others"
			}
		} else {
			option.Category = addCategory
		}
		option.Tags = []string{}
		if sitename != "" {
			option.Tags = append(option.Tags, client.GenerateTorrentTagFromSite(sitename))
			siteConfig := config.GetSiteConfig(sitename)
			if siteConfig.GlobalHnR {
				option.Tags = append(option.Tags, "_hr")
			}
		}
		option.Tags = append(option.Tags, fixedTags...)
		err = clientInstance.AddTorrent(torrentContent, option, nil)
		if err != nil {
			fmt.Printf("✕torrent (%d/%d) %s: failed to add to client (%v)\n", i+1, cntAll, torrentFile, err)
			errCnt++
			continue
		}
		if renameAdded {
			err := os.Rename(torrentFile, torrentFile+".added")
			if err != nil {
				log.Debugf("Failed to rename successfully added torrent %s to .added extension: %v", torrentFile, err)
			}
		} else if deleteAdded {
			err := os.Remove(torrentFile)
			if err != nil {
				log.Debugf("Failed to delete successfully added torrent %s: %v", torrentFile, err)
			}
		}
		cntAdded++
		sizeAdded += tinfo.Size
		fmt.Printf("✓torrent (%d/%d) %s: added to client\n", i+1, cntAll, torrentFile)
	}
	fmt.Printf("\nDone. Added torrent (Size/Cnt): %s / %d; ErrorCnt: %d\n", utils.BytesSize(float64(sizeAdded)), cntAdded, errCnt)
	clientInstance.Close()
	if errCnt > 0 {
		os.Exit(1)
	}
}
