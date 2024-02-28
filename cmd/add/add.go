package add

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
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
Args is torrent list that each one could be a local filename (e.g. "*.torrent" or "[M-TEAM]CLANNAD.torrent"),
torrent id (e.g.: "mteam.488424"), or torrent url (e.g.: "https://kp.m-team.cc/details.php?id=488424").
Use a single "-" as args to read torrent list from stdin, delimited by blanks,
as a special case, it also supports directly reading .torrent file contents from stdin.

--rename <name> flag supports the following variable placeholders:
* [size] : Torrent size
* [id] :  Torrent id in site
* [site] : Torrent site
* [filename] : Original torrent filename, with ".torrent" extension removed
* [name] : Torrent name`,
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
	command.Flags().StringVarP(&rename, "rename", "", "", "Rename added torrents (supports variables)")
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
	// directly read a torrent content from stdin.
	var directTorrentContent []byte
	torrents := util.ParseFilenameArgs(args[1:]...)
	if len(torrents) == 1 && torrents[0] == "-" {
		if config.InShell {
			return fmt.Errorf(`"-" arg can not be used in shell`)
		}
		if stdin, err := io.ReadAll(os.Stdin); err != nil {
			return fmt.Errorf("failed to read stdin: %v", err)
		} else if bytes.HasPrefix(stdin, []byte("d8:announce")) {
			// Matches with .torrent file magic number.
			// See: https://en.wikipedia.org/wiki/Torrent_file , https://en.wikipedia.org/wiki/Bencode .
			directTorrentContent = stdin
		} else if data, err := shlex.Split(string(stdin)); err != nil {
			return fmt.Errorf("failed to parse stdin to tokens: %v", err)
		} else {
			torrents = data
		}
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
	}
	var fixedTags []string
	if addTags != "" {
		fixedTags = util.SplitCsv(addTags)
	}
	domainSiteMap := map[string]string{}
	siteInstanceMap := map[string]site.Site{}
	errorCnt := int64(0)
	cntAdded := int64(0)
	sizeAdded := int64(0)
	cntAll := len(torrents)

	for i, torrent := range torrents {
		var siteName string
		var filename string // original torrent filename
		var content []byte
		var id string // site torrent id
		var err error
		var hr bool
		isLocal := forceLocal || torrent == "-" || !util.IsUrl(torrent) && strings.HasSuffix(torrent, ".torrent")

		if !isLocal {
			// site torrent
			siteName = defaultSite
			if !util.IsUrl(torrent) {
				i := strings.Index(torrent, ".")
				if i != -1 && i < len(torrent)-1 {
					siteName = torrent[:i]
				}
			} else {
				domain := util.GetUrlDomain(torrent)
				if domain == "" {
					fmt.Printf("✕add (%d/%d) %s: failed to parse domain", i+1, cntAll, torrent)
					errorCnt++
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
				fmt.Printf("✕add (%d/%d) %s: no site found or provided\n", i+1, cntAll, torrent)
				errorCnt++
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
			content, filename, id, err = siteInstance.DownloadTorrent(torrent)
		} else {
			if torrent == "-" {
				filename = ""
				content = directTorrentContent
			} else if strings.HasSuffix(torrent, ".added") {
				fmt.Printf("-skip (%d/%d) %s\n", i+1, cntAll, torrent)
				continue
			} else {
				filename = path.Base(torrent)
				content, err = os.ReadFile(torrent)
			}
		}

		if err != nil {
			fmt.Printf("✕add (%d/%d) %s (site=%s): failed to fetch: %v\n", i+1, cntAll, torrent, siteName, err)
			errorCnt++
			continue
		}
		tinfo, err := torrentutil.ParseTorrent(content, 99)
		if err != nil {
			fmt.Printf("✕add (%d/%d) %s (site=%s): failed to parse torrent: %v\n", i+1, cntAll, torrent, siteName, err)
			errorCnt++
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
		if rename != "" {
			basename := filename
			if i := strings.LastIndex(basename, "."); i != -1 {
				basename = basename[:i]
			}
			option.Name = rename
			option.Name = strings.ReplaceAll(option.Name, "[size]", util.BytesSize(float64(tinfo.Size)))
			option.Name = strings.ReplaceAll(option.Name, "[id]", id)
			option.Name = strings.ReplaceAll(option.Name, "[site]", siteName)
			option.Name = strings.ReplaceAll(option.Name, "[filename]", basename)
			option.Name = strings.ReplaceAll(option.Name, "[name]", tinfo.Info.Name)
		}
		err = clientInstance.AddTorrent(content, option, nil)
		if err != nil {
			fmt.Printf("✕add (%d/%d) %s (site=%s): failed to add torrent to client: %v // %s\n",
				i+1, cntAll, torrent, siteName, err, tinfo.ContentPath)
			errorCnt++
			continue
		}
		if isLocal && torrent != "-" {
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
		fmt.Printf("✓add (%d/%d) %s (site=%s). infoHash=%s // %s\n",
			i+1, cntAll, torrent, siteName, tinfo.InfoHash, tinfo.ContentPath)
	}
	fmt.Printf("\nDone. Added torrent (Size/Cnt): %s / %d; ErrorCnt: %d\n",
		util.BytesSize(float64(sizeAdded)), cntAdded, errorCnt)
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
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
