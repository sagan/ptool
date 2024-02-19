package add

import (
	"fmt"
	"io"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/site/tpl"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/torrentutil"
)

var command = &cobra.Command{
	Use:         "add {client} {torrentFilename | torrentId | torrentUrl}...",
	Aliases:     []string{"addlocal"},
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "add"},
	Short:       "Add torrents to client.",
	Long: `Add torrents to client.
Each arg could be a local filename (e.g. "*.torrent" or "[M-TEAM]CLANNAD (2007).torrent"),
torrent id (e.g.: "mteam.488424"), or torrent url (e.g.: "https://kp.m-team.cc/details.php?id=488424").`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE: add,
}

var (
	addCategoryAuto    = false
	addPaused          = false
	skipCheck          = false
	sequentialDownload = false
	renameAdded        = false
	deleteAdded        = false
	forceLocal         = false
	rename             = ""
	addCategory        = ""
	defaultSite        = ""
	addTags            = ""
	savePath           = ""
)

func init() {
	command.Flags().BoolVarP(&skipCheck, "skip-check", "", false, "Skip hash checking when adding torrents")
	command.Flags().BoolVarP(&addPaused, "add-paused", "", false, "Add torrents to client in paused state")
	command.Flags().BoolVarP(&addCategoryAuto, "add-category-auto", "", false,
		"Automatically set category of added torrent to corresponding sitename")
	command.Flags().BoolVarP(&sequentialDownload, "sequential-download", "", false,
		"(qbittorrent only) Enable sequential download")
	command.Flags().BoolVarP(&renameAdded, "rename-added", "", false,
		"Rename successfully added *.torrent file to *.torrent.added")
	command.Flags().BoolVarP(&deleteAdded, "delete-added", "", false, "Delete successfully added *.torrent file")
	command.Flags().BoolVarP(&forceLocal, "force-local", "", false, "Force treat all arg as local torrent filename")
	command.Flags().StringVarP(&rename, "rename", "", "", "Rename added torrent (for adding single torrent only)")
	command.Flags().StringVarP(&addCategory, "add-category", "", "", "Set category of added torrents")
	command.Flags().StringVarP(&savePath, "add-save-path", "", "", "Set save path of added torrents")
	command.Flags().StringVarP(&defaultSite, "site", "", "", "Set default site of added torrents")
	command.Flags().StringVarP(&addTags, "add-tags", "", "", "Add tags to added torrent (comma-separated)")
	cmd.RootCmd.AddCommand(command)
	command2.Flags().AddFlagSet(command.Flags())
	cmd.RootCmd.AddCommand(command2)
}

func add(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	if renameAdded && deleteAdded {
		return fmt.Errorf("--rename-added and --delete-added flags are NOT compatible")
	}
	torrents := util.ParseFilenameArgs(args[1:]...)
	if rename != "" && len(torrents) > 1 {
		return fmt.Errorf("--rename flag can only be used with exact one torrent arg")
	}
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}
	option := &client.TorrentOption{
		Pause:              addPaused,
		SavePath:           savePath,
		SkipChecking:       skipCheck,
		SequentialDownload: sequentialDownload,
		Name:               rename,
	}
	var fixedTags []string
	if addTags != "" {
		fixedTags = strings.Split(addTags, ",")
	}
	domainSiteMap := map[string]string{}
	siteInstanceMap := map[string]site.Site{}
	cntError := int64(0)
	cntAdded := int64(0)
	sizeAdded := int64(0)
	cntAll := len(torrents)

	for i, torrent := range torrents {
		var isLocal bool
		var siteName string
		var torrentContent []byte
		var err error
		var hr bool
		if forceLocal || torrent == "-" || !util.IsUrl(torrent) && strings.HasSuffix(torrent, ".torrent") {
			isLocal = true
		}
		if !isLocal {
			// site torrent
			isLocal = false
			siteName = defaultSite
			if !util.IsUrl(torrent) {
				i := strings.Index(torrent, ".")
				if i != -1 && i < len(torrent)-1 {
					siteName = torrent[:i]
					torrent = torrent[i+1:]
				}
			} else {
				domain := util.GetUrlDomain(torrent)
				if domain == "" {
					fmt.Printf("✕add (%d/%d) %s error: failed to parse domain", i+1, cntAll, torrent)
					cntError++
					continue
				}
				sitename := ""
				ok := false
				if sitename, ok = domainSiteMap[domain]; !ok {
					domainSiteMap[domain], err = tpl.GuessSiteByDomain(domain, defaultSite)
					if err != nil {
						log.Warnf("Failed to find match site for %s: %v", domain, err)
					}
					sitename = domainSiteMap[domain]
				}
				if sitename == "" {
					log.Warnf("Torrent %s: url does not match any site. will use provided default site", torrent)
				} else {
					siteName = sitename
				}
			}
			if siteName == "" {
				fmt.Printf("✕add (%d/%d) %s error: no site found or provided\n", i+1, cntAll, torrent)
				cntError++
				continue
			}
			if siteInstanceMap[siteName] == nil {
				siteInstance, err := site.CreateSite(siteName)
				if err != nil {
					return fmt.Errorf("failed to create site %s: %v", siteName, err)
				}
				siteInstanceMap[siteName] = siteInstance
			}
			siteInstance := siteInstanceMap[siteName]
			hr = siteInstance.GetSiteConfig().GlobalHnR
			torrentContent, _, err = siteInstance.DownloadTorrent(torrent)
		} else {
			isLocal = true
			if strings.HasSuffix(torrent, ".added") {
				fmt.Printf("-skip (%d/%d) %s\n", i+1, cntAll, torrent)
				continue
			}
			if torrent == "-" {
				torrentContent, err = io.ReadAll(os.Stdin)
			} else {
				torrentContent, err = os.ReadFile(torrent)
			}
		}

		if err != nil {
			fmt.Printf("✕add (%d/%d) %s (site=%s) error: failed to get torrent: %v\n",
				i+1, cntAll, torrent, siteName, err)
			cntError++
			continue
		}
		tinfo, err := torrentutil.ParseTorrent(torrentContent, 99)
		if err != nil {
			fmt.Printf("✕add (%d/%d) %s (site=%s) error: failed to parse torrent: %v\n",
				i+1, cntAll, torrent, siteName, err)
			cntError++
			continue
		}
		if siteName == "" {
			if sitename, err := tpl.GuessSiteByTrackers(tinfo.Trackers, defaultSite); err != nil {
				log.Warnf("Failed to find match site for %s by trackers: %v", torrent, err)
			} else {
				siteName = sitename
			}
		}
		if addCategoryAuto {
			if siteName != "" {
				option.Category = siteName
			} else if addCategory != "" {
				option.Category = addCategory
			} else {
				option.Category = "Others"
			}
		} else {
			option.Category = addCategory
		}
		if siteName != "" {
			option.Tags = append(option.Tags, client.GenerateTorrentTagFromSite(siteName))
		}
		if hr {
			option.Tags = append(option.Tags, "_hr")
		}
		option.Tags = append(option.Tags, fixedTags...)
		err = clientInstance.AddTorrent(torrentContent, option, nil)
		if err != nil {
			fmt.Printf("✕add (%d/%d) %s (site=%s) error: failed to add torrent to client: %v // %s\n",
				i+1, cntAll, torrent, siteName, err, tinfo.ContentPath)
			cntError++
			continue
		}
		if isLocal {
			if renameAdded {
				if err := os.Rename(torrent, torrent+".added"); err != nil {
					log.Debugf("Failed to rename %s to *.added: %v // %s", torrent, err, tinfo.ContentPath)
				}
			} else if deleteAdded {
				if err := os.Remove(torrent); err != nil {
					log.Debugf("Failed to delete %s: %v // %s", torrent, err, tinfo.ContentPath)
				}
			}
		}
		cntAdded++
		sizeAdded += tinfo.Size
		fmt.Printf("✓add (%d/%d) %s (site=%s) success. infoHash=%s // %s\n",
			i+1, cntAll, torrent, siteName, tinfo.InfoHash, tinfo.ContentPath)
	}
	fmt.Printf("\nDone. Added torrent (Size/Cnt): %s / %d; ErrorCnt: %d\n",
		util.BytesSize(float64(sizeAdded)), cntAdded, cntError)
	if cntError > 0 {
		return fmt.Errorf("%d errors", cntError)
	}
	return nil
}

var command2 = &cobra.Command{
	Use:   "add2 [args]",
	Short: `Alias of "add --add-category-auto --sequential-download [args]"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		addCategoryAuto = true
		sequentialDownload = true
		return command.RunE(cmd, args)
	},
}
