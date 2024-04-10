package public

import (
	"net/url"
	"regexp"

	"github.com/sagan/ptool/util"
)

// public Bittorrent sites. add cmd supports adding torrent url of these sites

type PublicBittorrentSite struct {
	Name                    string
	Domains                 []string // first one of Domains is considered as primary domain
	TrackerDomains          []string
	TorrentDownloadUrl      string         // placeholders: {{origin}}, {{domain}}, {{id}}
	TorrentUrlIdRegexp      *regexp.Regexp // extract torrent id (in "id" subgroup) from url
	TorrentDownloadInterval int64          // min interval between downloading 2 torrents (miliseconds)
}

var (
	Sites = []*PublicBittorrentSite{
		{
			Name:    "nyaa",
			Domains: []string{"sukebei.nyaa.si", "nyaa.si"},
			// https://sukebei.nyaa.si/upload
			// https://nyaa.si/upload
			TrackerDomains:          []string{"sukebei.tracker.wf", "nyaa.tracker.wf"},
			TorrentUrlIdRegexp:      regexp.MustCompile(`\bview/(?P<id>\d+)\b`),
			TorrentDownloadUrl:      `{{origin}}/download/{{id}}.torrent`,
			TorrentDownloadInterval: 3000, // nyaa 限流。每几秒钟内只能下载1个种子
		},
	}
	// site name & domain (main or tracker) => site
	SitesMap = map[string]*PublicBittorrentSite{}
)

func init() {
	for _, btsite := range Sites {
		SitesMap[btsite.Name] = btsite
		for _, domain := range btsite.Domains {
			SitesMap[domain] = btsite
		}
		for _, domain := range btsite.TrackerDomains {
			SitesMap[domain] = btsite
		}
	}
}

// Get a site by a website or tracker url or domain.
func GetSiteByDomain(defaultSite string, domainOrUrls ...string) *PublicBittorrentSite {
	if SitesMap[defaultSite] != nil {
		return SitesMap[defaultSite]
	}
	for _, domain := range domainOrUrls {
		if util.IsUrl(domain) {
			if urlObj, err := url.Parse(domain); err != nil {
				continue
			} else {
				domain = urlObj.Hostname()
			}
		}
		if SitesMap[domain] != nil {
			return SitesMap[domain]
		}
	}
	return nil
}
