package tpl

// 站点模板

import (
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
)

var (
	SITES = map[string](*config.SiteConfigStruct){
		"0ff": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://pt.0ff.cc/",
		},
		"1ptba": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://1ptba.com/",
		},
		"2xfree": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://pt.2xfree.org/",
		},
		"3wmg": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://www.3wmg.com/",
		},
		"52pt": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://52pt.site/",
		},
		"audiences": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://audiences.me/",
		},
		"azusa": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://azusa.wiki/",
		},
		"beitai": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://www.beitai.pt/",
		},
		"btschool": &config.SiteConfigStruct{
			Type:      "nexusphp",
			Url:       "https://pt.btschool.club/",
			GlobalHnR: true,
		},
		"carpt": &config.SiteConfigStruct{
			Type:      "nexusphp",
			Url:       "https://carpt.net/",
			GlobalHnR: true,
		},
		"cyanbug": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://cyanbug.net/",
		},
		"discfan": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://discfan.net/",
		},
		"gainbound": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://gainbound.net/",
		},
		"haidan": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://www.haidan.video/",
		},
		"hdatmos": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://hdatmos.club/",
		},
		"hddolby": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://www.hddolby.com/",
		},
		"hdfans": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://hdfans.org/",
		},
		"hdmayi": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "http://hdmayi.com/",
		},
		"hdcity": &config.SiteConfigStruct{
			Type:                       "nexusphp",
			Url:                        "https://hdcity.city/",
			SelectorTorrentDetailsLink: `a[href^="t-"]`,
			SelectorTorrentTime:        `.trtop > div:nth-last-child(2)@text`,
			SelectorTorrentSize:        `.trbo > div:nth-child(3)@text`,
			SelectorTorrentSeeders:     `a[title="种子数"] font`,
			SelectorTorrentLeechers:    ``,
			SelectorTorrentSnatched:    `a[title="完成数"]@text`,
		},
		"hdtime": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://hdtime.org/",
		},
		"hdupt": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://pt.hdupt.com/",
		},
		"hdvideo": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://hdvideo.one/",
		},
		"hdzone": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://hdzone.me/",
		},
		"hhanclub": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://hhanclub.top/",
		},
		"icc2022": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://www.icc2022.com/",
		},
		"joyhd": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://www.joyhd.net/",
		},
		"kamept": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://kamept.com/",
		},
		"leaves": &config.SiteConfigStruct{
			Type:              "nexusphp",
			Url:               "https://leaves.red/",
			TorrentsExtraUrls: []string{"https://leaves.red/special.php"},
		},
		"lemonhd": &config.SiteConfigStruct{
			Type:        "nexusphp",
			Url:         "https://lemonhd.org/",
			TorrentsUrl: "https://lemonhd.org/torrents_new.php",
		},
		"mteam": &config.SiteConfigStruct{
			Type:              "nexusphp",
			Url:               "https://kp.m-team.cc/",
			TorrentsExtraUrls: []string{"https://kp.m-team.cc/adult.php"},
		},
		"nicept": &config.SiteConfigStruct{
			Type:      "nexusphp",
			Url:       "https://www.nicept.net/",
			GlobalHnR: true,
		},
		"oshen": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://www.oshen.win/",
		},
		"piggo": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://piggo.me/",
		},
		"ptchina": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://ptchina.org/",
		},
		"ptsbao": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://ptsbao.club/",
		},
		"pthome": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://pthome.net/",
		},
		"pttime": &config.SiteConfigStruct{
			Type:              "nexusphp",
			Url:               "https://www.pttime.org/",
			TorrentsExtraUrls: []string{"https://www.pttime.org/adults.php"},
		},
		"sharkpt": &config.SiteConfigStruct{
			Type:                      "nexusphp",
			Url:                       "https://sharkpt.net/",
			SelectorTorrent:           ".torrent-action-bookmark",
			SelectorTorrentProcessBar: ".torrent-progress",
			SelectorUserInfo:          ".m_nav",
		},
		"soulvoice": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://pt.soulvoice.club/",
		},
		"u2": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://u2.dmhy.org/",
		},
		"wintersakura": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://wintersakura.net/",
		},
		"xinglin": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://xinglin.one/",
		},
		"zmpt": &config.SiteConfigStruct{
			Type: "nexusphp",
			Url:  "https://zmpt.cc/",
		},
	}
)

func init() {
	for name := range SITES {
		site.Register(&site.RegInfo{
			Name:    name,
			Creator: create,
		})
	}
}

func create(name string, siteConfig *config.SiteConfigStruct, globalConfig *config.ConfigStruct) (
	site.Site, error) {
	sc := *siteConfig
	utils.Assign(&sc, SITES[name])
	return site.CreateSiteInternal(name, &sc, globalConfig)
}
