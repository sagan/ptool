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
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/torrentutil"
)

var command = &cobra.Command{
	Use:         "dltorrent {torrentId | torrentUrl}... [--dir dir]",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "dltorrent"},
	Short:       "Download site torrents to local.",
	Long: `Download site torrents to local.

--filename <name> flag supports the following variable placeholders:
* [size] : Torrent size
* [id] :  Torrent id in site
* [site] : Torrent site
* [filename] : Original torrent filename, with ".torrent" extension removed
* [name] : Torrent name`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: dltorrent,
}

var (
	downloadDir = ""
	savename    = ""
	defaultSite = ""
)

func init() {
	command.Flags().StringVarP(&defaultSite, "site", "", "", "Set default site of torrents")
	command.Flags().StringVarP(&downloadDir, "dir", "", ".", `Set the dir of downloaded torrents`)
	command.Flags().StringVarP(&savename, "filename", "", "",
		"Set the filename of downloaded torrents (supports variables)")
	cmd.RootCmd.AddCommand(command)
}

func dltorrent(cmd *cobra.Command, args []string) error {
	errorCnt := int64(0)
	torrentIds := args
	siteInstanceMap := map[string]site.Site{}
	domainSiteMap := map[string]string{}
	var err error

	for _, torrentId := range torrentIds {
		siteName := defaultSite
		if !util.IsUrl(torrentId) {
			i := strings.Index(torrentId, ".")
			if i != -1 && i < len(torrentId)-1 {
				siteName = torrentId[:i]
				torrentId = torrentId[i+1:]
			}
		} else {
			domain := util.GetUrlDomain(torrentId)
			if domain == "" {
				fmt.Printf("✕download %s: failed to parse domain", torrentId)
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
				log.Warnf("Torrent %s: url does not match any site. will use provided default site", torrentId)
			} else {
				siteName = sitename
			}
		}
		if siteName == "" {
			fmt.Printf("✕download %s: no site provided\n", torrentId)
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
		content, filename, id, err := siteInstance.DownloadTorrent(torrentId)
		if err != nil {
			fmt.Printf("✕download %s (site=%s): failed to fetch: %v\n", torrentId, siteName, err)
			errorCnt++
			continue
		}
		tinfo, err := torrentutil.ParseTorrent(content, 99)
		if err != nil {
			fmt.Printf("✕download %s (site=%s): failed to parse torrent: %v\n", torrentId, siteName, err)
			errorCnt++
			continue
		}
		fileName := ""
		if savename == "" {
			fileName = filename
		} else {
			fileName = savename
			basename := filename
			if i := strings.LastIndex(basename, "."); i != -1 {
				basename = basename[:i]
			}
			fileName = strings.ReplaceAll(fileName, "[size]", util.BytesSize(float64(tinfo.Size)))
			fileName = strings.ReplaceAll(fileName, "[id]", id)
			fileName = strings.ReplaceAll(fileName, "[site]", siteName)
			fileName = strings.ReplaceAll(fileName, "[filename]", basename)
			fileName = strings.ReplaceAll(fileName, "[name]", tinfo.Info.Name)
		}
		err = os.WriteFile(downloadDir+"/"+fileName, content, 0666)
		if err != nil {
			fmt.Printf("✕download %s (site=%s): failed to save to %s/: %v\n", fileName, siteName, downloadDir, err)
			errorCnt++
		} else {
			fmt.Printf("✓download %s (site=%s): saved to %s/\n", fileName, siteName, downloadDir)
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
