package xseedadd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/helper"
)

var command = &cobra.Command{
	Use:         "xseedadd {client} {torrentFilename | torrentId | torrentUrl}...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "xseedadd"},
	Short:       "Add xseed torrents to client.",
	Long: `Add xseed torrents to client.
Args is torrent list that each one could be a local filename (e.g. "*.torrent" or "[M-TEAM]CLANNAD.torrent"),
site torrent id (e.g.: "mteam.488424") or url (e.g.: "https://kp.m-team.cc/details.php?id=488424").
Torrent url that does NOT belong to any site (e.g.: a public site url) is also supported.
Use a single "-" to read .torrent file contents from stdin.

For every torrent in the list (the "xseed torrent"), it will try to find the existing target torrent in the client,
and add this torrent to the client as the corresponding xseed torrent of the target torrent.
To be qualified as the target torrent, the existing torrent must have the same contents (name & size of files)
with this xseed torrent, is fullly completed downloaded, and is in seeding state currently.
If no target torrent for a xseed torrent is found in the client, it will NOT add the xseed torrent to client.

If a torrent of the list already exists in client, it will also be skipped.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE: xseedadd,
}

var (
	renameAdded = false
	deleteAdded = false
	addPaused   = false
	check       = false
	dryRun      = false
	forceLocal  = false
	addCategory = ""
	addTags     = ""
	defaultSite = ""
	category    = ""
	tag         = ""
	filter      = ""
)

func init() {
	command.Flags().BoolVarP(&renameAdded, "rename-added", "", false,
		"Rename successfully added torrent file to *"+constants.FILENAME_SUFFIX_ADDED)
	command.Flags().BoolVarP(&deleteAdded, "delete-added", "", false, "Delete successfully added torrent file")
	command.Flags().BoolVarP(&addPaused, "add-paused", "", false, "Add xseed torrents to client in paused state")
	command.Flags().BoolVarP(&check, "check", "", false, "Let client do hash checking when adding xseed torrents")
	command.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Dry run. Do NOT actually add xseed torrents to client")
	command.Flags().BoolVarP(&forceLocal, "force-local", "", false, "Force treat all args as local torrent filename")
	command.Flags().StringVarP(&defaultSite, "site", "", "", "Set default site of torrent url")
	command.Flags().StringVarP(&addCategory, "add-category", "", "",
		"Manually set category of added xseed torrent. By Default it uses the original torrent's")
	command.Flags().StringVarP(&addTags, "add-tags", "", "", "Set tags of added xseed torrent (comma-separated)")
	command.Flags().StringVarP(&category, "category", "", "", "Only xseed torrents that belongs to this category")
	command.Flags().StringVarP(&tag, "tag", "", "",
		"Comma-separated list. Only xseed torrents which tags contain any one in the list")
	command.Flags().StringVarP(&filter, "filter", "", "", "Only xseed torrents which name contains this")
	cmd.RootCmd.AddCommand(command)
}

func xseedadd(cmd *cobra.Command, args []string) error {
	if renameAdded && deleteAdded {
		return fmt.Errorf("--rename-added and --delete-added flags are NOT compatible")
	}
	clientName := args[0]
	torrents := util.ParseFilenameArgs(args[1:]...)
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}
	clientTorrents, err := clientInstance.GetTorrents("seeding", category, true)
	if err != nil {
		return fmt.Errorf("failed to get client torrents: %v", err)
	}
	var fixedTags []string
	if addTags != "" {
		fixedTags = util.SplitCsv(addTags)
	}
	clientTorrents = util.Filter(clientTorrents, func(t client.Torrent) bool {
		return t.IsFullComplete() && !t.HasTag(config.NOXSEED_TAG) && (tag == "" || t.HasAnyTag(tag))
	})
	sort.Slice(clientTorrents, func(i, j int) bool {
		if clientTorrents[i].Size != clientTorrents[j].Size {
			return clientTorrents[i].Size > clientTorrents[j].Size
		}
		a, b := 0, 0
		if clientTorrents[i].Category == config.XSEED_TAG || clientTorrents[i].HasTag(config.XSEED_TAG) {
			a = 1
		}
		if clientTorrents[j].Category == config.XSEED_TAG || clientTorrents[j].HasTag(config.XSEED_TAG) {
			b = 1
		}
		if a != b {
			return a < b
		}
		if clientTorrents[i].Name != clientTorrents[j].Name {
			return clientTorrents[i].Name < clientTorrents[j].Name
		}
		return clientTorrents[i].InfoHash < clientTorrents[j].InfoHash
	})
	errorCnt := int64(0)
	for _, torrent := range torrents {
		content, tinfo, _, sitename, _, _, isLocal, err :=
			helper.GetTorrentContent(torrent, defaultSite, forceLocal, false, nil, false, nil)
		if err != nil {
			fmt.Printf("X%s: failed to get: %v\n", torrent, err)
			errorCnt++
			continue
		}
		if t, _ := clientInstance.GetTorrent(tinfo.InfoHash); t != nil {
			fmt.Printf("!%s: already exists in client as %s (%s)\n", torrent, t.InfoHash, t.Name)
			continue
		}
		var matchClientTorrent *client.Torrent
		for _, clientTorrent := range clientTorrents {
			if clientTorrent.Size > tinfo.Size {
				continue
			} else if clientTorrent.Size < tinfo.Size {
				break
			}
			clientTorrentContents, err := clientInstance.GetTorrentContents(clientTorrent.InfoHash)
			if err != nil {
				log.Debugf("failed to get client torrent contents info: %v", err)
				continue
			}
			compareResult := tinfo.XseedCheckWithClientTorrent(clientTorrentContents)
			if compareResult == 0 {
				log.Debugf("Torrent %s has the same contents with client %s torrent.\n", torrent, clientName)
			} else if compareResult == 1 {
				log.Debugf("Torrent %s has the same (partial) contents with client %s torrent.\n", torrent, clientName)
			} else if compareResult == -2 {
				log.Debugf("Torrent %s has the DIFFERENT root folder, but same contents with client %s torrent.\n",
					torrent, clientName)
			} else {
				log.Debugf("Torrent %s does NOT has the same contents with client %s torrent.\n", torrent, clientName)
			}
			if compareResult >= 0 {
				matchClientTorrent = &clientTorrent
				break
			}
		}
		if matchClientTorrent == nil {
			fmt.Printf("X%s: no matched target torrent found in client\n", torrent)
			errorCnt++
			continue
		}
		if dryRun {
			fmt.Printf("✓%s: matches with client torrent %s (%s) (dry-run)\n",
				torrent, matchClientTorrent.InfoHash, matchClientTorrent.Name)
			continue
		}
		category := matchClientTorrent.Category
		if addCategory != "" {
			category = addCategory
		}
		tags := []string{config.XSEED_TAG}
		if sitename != "" {
			tags = append(tags, client.GenerateTorrentTagFromSite(sitename))
		}
		tags = append(tags, fixedTags...)
		err = clientInstance.AddTorrent(content, &client.TorrentOption{
			SavePath:     matchClientTorrent.SavePath,
			Category:     category,
			Tags:         tags,
			Pause:        addPaused,
			SkipChecking: !check,
		}, nil)
		if err != nil {
			fmt.Printf("X%s: matched with client torrent %s (%s), but failed to add to client: %v\n",
				torrent, matchClientTorrent.InfoHash, matchClientTorrent.Name, err)
			errorCnt++
		} else {
			fmt.Printf("✓%s: matched with client torrent %s (%s), added to client, save path: %s\n",
				torrent, matchClientTorrent.InfoHash, matchClientTorrent.Name, matchClientTorrent.SavePath)
			if isLocal && torrent != "-" {
				if renameAdded && !strings.HasSuffix(torrent, constants.FILENAME_SUFFIX_ADDED) {
					if err := os.Rename(torrent, util.TrimAnySuffix(torrent,
						constants.ProcessedFilenameSuffixes...)+constants.FILENAME_SUFFIX_ADDED); err != nil {
						log.Debugf("Failed to rename %s to *%s: %v", torrent, constants.FILENAME_SUFFIX_ADDED, err)
					}
				} else if deleteAdded {
					if err := os.Remove(torrent); err != nil {
						log.Debugf("Failed to delete %s: %v", torrent, err)
					}
				}
			}
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
