package dltorrent

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/site/tpl"
	"github.com/sagan/ptool/utils"
)

var command = &cobra.Command{
	Use:   "dltorrent <torrentIdOrUrl>...",
	Short: "Download site torrents to local.",
	Long:  `Download site torrents to local.`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE:  dltorrent,
}

var (
	downloadDir = ""
	defaultSite = ""
)

func init() {
	command.Flags().StringVarP(&defaultSite, "site", "", "", "Set default site of torrents")
	command.Flags().StringVarP(&downloadDir, "download-dir", "", ".", "Set the local dir of downloaded torrents. Default == current dir")
	cmd.RootCmd.AddCommand(command)
}

func dltorrent(cmd *cobra.Command, args []string) error {
	errorCnt := int64(0)
	torrentIds := args
	siteInstanceMap := map[string](site.Site){}
	domainSiteMap := map[string](string){}

	for _, torrentId := range torrentIds {
		siteName := defaultSite
		if !utils.IsUrl(torrentId) {
			i := strings.Index(torrentId, ".")
			if i != -1 && i < len(torrentId)-1 {
				siteName = torrentId[:i]
				torrentId = torrentId[i+1:]
			}
		} else {
			domain := utils.GetUrlDomain(torrentId)
			if domain == "" {
				fmt.Printf("torrent %s: failed to parse domain", torrentId)
				continue
			}
			sitename := ""
			ok := false
			if sitename, ok = domainSiteMap[domain]; !ok {
				domainSiteMap[domain] = tpl.GuessSiteByDomain(domain, defaultSite)
				sitename = domainSiteMap[domain]
			}
			if sitename == "" {
				log.Warnf("torrent %s: url does not match any site. will use provided default site", torrentId)
			} else {
				siteName = sitename
			}
		}
		if siteName == "" {
			fmt.Printf("torrent %s: no site provided\n", torrentId)
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
		torrentContent, filename, err := siteInstance.DownloadTorrent(torrentId)
		if err != nil {
			fmt.Printf("add site %s torrent %s error: failed to get site torrent: %v\n", siteInstance.GetName(), torrentId, err)
			errorCnt++
			continue
		}
		err = os.WriteFile(downloadDir+"/"+filename, torrentContent, 0777)
		if err != nil {
			fmt.Printf("torrent %s: failed to download to %s/: %v\n", filename, downloadDir, err)
			errorCnt++
		} else {
			fmt.Printf("torrent %s: downloaded to %s/\n", filename, downloadDir)
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
