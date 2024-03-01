package helper

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/Noooste/azuretls-client"
	log "github.com/sirupsen/logrus"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/site/tpl"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/torrentutil"
)

type PublicBittorrentSite struct {
	Name               string
	Domains            []string       // first one of Domains is considered as primary domain
	TorrentDownloadUrl string         // placeholders: {{origin}}, {{domain}}, {{id}}
	TorrentUrlIdRegexp *regexp.Regexp // extract torrent id (in "id" subgroup) from url
}

var (
	domainSiteMap = map[string]string{}

	// 公开 BT 网站，支持直接从这些网站下载种子
	knownPublicBittorrentSites = []*PublicBittorrentSite{
		{
			Name:               "nyaa",
			Domains:            []string{"sukebei.nyaa.si", "nyaa.si"},
			TorrentUrlIdRegexp: regexp.MustCompile(`\bview/(?P<id>\d+)\b`),
			TorrentDownloadUrl: `{{origin}}/download/{{id}}.torrent`,
		},
	}
	knownPublicBittorrentSitesMap = map[string]*PublicBittorrentSite{}
)

func init() {
	for _, btsite := range knownPublicBittorrentSites {
		for _, domain := range btsite.Domains {
			knownPublicBittorrentSitesMap[domain] = btsite
		}
	}
}

// Read a torrent and return it's contents. torrent could be: local filename (e.g.: abc.torrent),
// site torrent id (e.g.: mteam.1234) or url (e.g. https://kp.m-team.cc/details.php?id=488424),
// or "-" to read torrent contents from os.Stdin.
// isLocal: force treat torrent as local filename.
// stdin: if not nil, use it as torrent contents when torrent == "-" instead of reading from os.Stdin.
// siteInstance : if torrent is a site torrent, the created corresponding site instance.
// sitename & filename & id : if torrent is a site torrent, the downloaded torrent sitename & filename & id.
func GetTorrentContent(torrent string, defaultSite string, forceLocal bool, forceRemote bool, stdin []byte) (
	content []byte, tinfo *torrentutil.TorrentMeta, siteInstance site.Site, siteName string,
	filename string, id string, err error) {
	isLocal := !forceRemote && (forceLocal || torrent == "-" ||
		!util.IsUrl(torrent) && strings.HasSuffix(torrent, ".torrent"))
	if !isLocal {
		// site torrent
		siteName = defaultSite
		if !util.IsUrl(torrent) {
			if i := strings.Index(torrent, "."); i != -1 {
				siteName = torrent[:i]
			}
		} else if domain := util.GetUrlDomain(torrent); domain == "" {
			err = fmt.Errorf("failed to parse torrent domain")
			return
		} else {
			sitename := ""
			ok := false
			if sitename, ok = domainSiteMap[domain]; !ok {
				domainSiteMap[domain], err = tpl.GuessSiteByDomain(domain, defaultSite)
				if err != nil {
					log.Tracef("Failed to find match site for %s: %v", domain, err)
				}
				sitename = domainSiteMap[domain]
			}
			if sitename == "" {
				log.Tracef("Torrent %s: url does not match any site. will use provided default site", torrent)
			} else {
				siteName = sitename
			}
		}
		if siteName == "" {
			log.Tracef("%s: no site found, try to download torrent from url directly", torrent)
			var urlObj *url.URL
			var httpClient *azuretls.Session
			var headers [][]string
			if urlObj, err = url.Parse(torrent); err != nil {
				err = fmt.Errorf("%s: no site found and url is invalid", torrent)
			} else if httpClient, headers, err = site.CreateSiteHttpClient(&config.SiteConfigStruct{},
				config.Get()); err != nil {
				err = fmt.Errorf("%s: no site found and failed to create public http client", torrent)
			} else {
				downloadUrl := torrent
				if btsite := knownPublicBittorrentSitesMap[urlObj.Hostname()]; btsite != nil {
					if btsite.TorrentUrlIdRegexp != nil {
						if m := btsite.TorrentUrlIdRegexp.FindStringSubmatch(torrent); m != nil {
							id = m[btsite.TorrentUrlIdRegexp.SubexpIndex("id")]
							downloadUrl = btsite.TorrentDownloadUrl
							downloadUrl = strings.ReplaceAll(downloadUrl, "{{id}}", id)
							downloadUrl = strings.ReplaceAll(downloadUrl, "{{domain}}", btsite.Domains[0])
							downloadUrl = strings.ReplaceAll(downloadUrl, "{{origin}}", urlObj.Scheme+"://"+urlObj.Host)
						}
					}
					log.Tracef("found public bittorrent site %s torrent download url %s", btsite.Name, downloadUrl)
				}
				var res *azuretls.Response
				var header http.Header
				if res, header, err = util.FetchUrlWithAzuretls(downloadUrl, httpClient, "", "", headers); err != nil {
					err = fmt.Errorf("%s: failed to fetch: %v", torrent, err)
				} else {
					content = res.Body
					filename = util.ExtractFilenameFromHttpHeader(header)
				}
			}
		} else {
			siteInstance, err = site.CreateSite(siteName)
			if err != nil {
				err = fmt.Errorf("failed to create site %s: %v", siteName, err)
				return
			}
			content, filename, id, err = siteInstance.DownloadTorrent(torrent)
		}
	} else {
		if torrent == "-" {
			filename = ""
			if stdin != nil {
				content = stdin
			} else if config.InShell {
				err = fmt.Errorf(`"-" arg can not be used in shell`)
			} else {
				content, err = io.ReadAll(os.Stdin)
			}
		} else if strings.HasSuffix(torrent, ".added") {
			err = fmt.Errorf(".added file is skipped")
		} else {
			filename = path.Base(torrent)
			content, err = os.ReadFile(torrent)
		}
	}
	if err != nil {
		return
	}
	if tinfo, err = torrentutil.ParseTorrent(content, 99); err != nil {
		err = fmt.Errorf("%s: failed to parse torrent: %v", torrent, err)
		return
	}
	if siteName == "" {
		if sitename, err := tpl.GuessSiteByTrackers(tinfo.Trackers, defaultSite); err != nil {
			log.Warnf("Failed to find match site for %s by trackers: %v", torrent, err)
		} else {
			siteName = sitename
		}
	}
	return
}
