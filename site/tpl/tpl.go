package tpl

// 站点模板

import (
	"net/url"
	"sort"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
	"golang.org/x/exp/slices"
)

var (
	SITES = map[string](*config.SiteConfigStruct){
		"0ff": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Aliases: []string{"pt0ffcc"},
			Url:     "https://pt.0ff.cc/",
			Comment: "自由农场",
		},
		"1ptba": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://1ptba.com/",
			Comment: "1PTA (壹PT吧)",
		},
		"2xfree": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Aliases: []string{"pt2xfree"},
			Url:     "https://pt.2xfree.org/",
			Comment: "2xFree",
		},
		"3wmg": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://www.3wmg.com/",
			Comment: "芒果",
		},
		"52pt": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://52pt.site/",
			Comment: "52PT",
		},
		"audiences": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://audiences.me/",
			Comment: "观众",
		},
		"azusa": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://azusa.wiki/",
			Comment: "梓喵",
		},
		"beitai": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://www.beitai.pt/",
			Comment: "备胎",
		},
		"btschool": &config.SiteConfigStruct{
			Type:      "nexusphp",
			Url:       "https://pt.btschool.club/",
			GlobalHnR: true,
			Comment:   "学校",
		},
		"carpt": &config.SiteConfigStruct{
			Type:      "nexusphp",
			Url:       "https://carpt.net/",
			GlobalHnR: true,
			Comment:   "CarPT (小车站)",
		},
		"cyanbug": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://cyanbug.net/",
			Comment: "大青虫",
		},
		"dhtclub": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://pt.dhtclub.com/",
			Comment: "DHTCLUB PT",
		},
		"discfan": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://discfan.net/",
			Comment: "蝶粉",
		},
		"gainbound": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://gainbound.net/",
			Comment: "丐帮",
		},
		"gamegamept": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Aliases: []string{"ggpt"},
			Url:     "https://www.gamegamept.com/",
			Comment: "GGPT",
		},
		"gtk": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Aliases: []string{"ptgtk"},
			Url:     "https://pt.gtk.pw/",
			Comment: "PT GTK",
		},
		"haidan": &config.SiteConfigStruct{
			Type:                       "nexusphp",
			Url:                        "https://www.haidan.video/",
			SelectorTorrentsListHeader: `none`, // do NOT exists
			SelectorTorrentsList:       `.torrent_panel_inner`,
			SelectorTorrentBlock:       `.torrent_wrap`,
			SelectorTorrentTime:        `.time_col span:last-child`,
			Comment:                    "海胆",
		},
		"hdarea": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://www.hdarea.co/",
			Comment: "高清地带",
		},
		"hdatmos": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://hdatmos.club/",
			Comment: "阿童木",
		},
		"hddolby": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://www.hddolby.com/",
			Comment: "杜比",
		},
		"hdfans": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://hdfans.org/",
			Comment: "红豆饭",
		},
		"hdhome": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://hdhome.org/",
			Comment: "家园",
		},
		"hdcity": &config.SiteConfigStruct{
			Type:                       "nexusphp",
			Url:                        "https://hdcity.city/",
			SearchUrl:                  "https://hdcity.city/pt?iwannaseethis=%s",
			SelectorTorrentDetailsLink: `a[href^="t-"]`,
			SelectorTorrentTime:        `.trtop > div:nth-last-child(2)@text`,
			SelectorTorrentSize:        `.trbo > div:nth-child(3)@text`,
			SelectorTorrentSeeders:     `a[title="种子数"] font`,
			SelectorTorrentLeechers:    ``,
			SelectorTorrentSnatched:    `a[title="完成数"]@text`,
			SelectorUserInfoUserName:   `#bottomnav a[href="userdetails"] strong`,
			SelectorUserInfoUploaded:   `#bottomnav a[href="userdetails"] i[title="上传量："]@after`,
			SelectorUserInfoDownloaded: `#bottomnav a[href="userdetails"] i[title="下载量："]@after`,
			Comment:                    "城市",
		},
		"hdmayi": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "http://hdmayi.com/",
			Comment: "蚂蚁",
		},
		"hdtime": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://hdtime.org/",
			Comment: "高清时光",
		},
		"hdupt": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Aliases: []string{"upxin"},
			Url:     "https://pt.hdupt.com/",
			Comment: "好多油",
		},
		"hdvideo": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://hdvideo.one/",
			Comment: "高清视频",
		},
		"hdzone": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://hdzone.me/",
			Comment: "高清地带",
		},
		"hhanclub": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Aliases: []string{"hh"},
			Url:     "https://hhanclub.top/",
			Comment: "憨憨",
		},
		"icc2022": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Aliases: []string{"icc"},
			Url:     "https://www.icc2022.com/",
			Comment: "冰淇淋",
		},
		"ilolicon": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://share.ilolicon.com/",
			Comment: "ilolicon PT",
		},
		"itzmx": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://pt.itzmx.com/",
			Comment: "PT分享站",
		},
		"joyhd": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://www.joyhd.net/",
			Comment: "JoyHD",
		},
		"kamept": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://kamept.com/",
			Comment: "KamePT",
		},
		"leaves": &config.SiteConfigStruct{
			Type:              "nexusphp",
			Aliases:           []string{"redleaves"},
			Url:               "https://leaves.red/",
			TorrentsExtraUrls: []string{"https://leaves.red/special.php"},
			SearchUrl:         `https://leaves.red/search.php?search=%s&search_area=0`,
			Comment:           "红叶",
		},
		"lemonhd": &config.SiteConfigStruct{
			Type:        "nexusphp",
			Aliases:     []string{"leaguehd"},
			Url:         "https://lemonhd.org/",
			TorrentsUrl: "https://lemonhd.org/torrents_new.php",
			Comment:     "柠檬",
		},
		"m-team": &config.SiteConfigStruct{
			Type:              "nexusphp",
			Aliases:           []string{"mteam"},
			Url:               "https://kp.m-team.cc/",
			TorrentsExtraUrls: []string{"https://kp.m-team.cc/adult.php"},
			Comment:           "馒头",
		},
		"nicept": &config.SiteConfigStruct{
			Type:      "nexusphp",
			Url:       "https://www.nicept.net/",
			GlobalHnR: true,
			Comment:   "老师",
		},
		"oshen": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://www.oshen.win/",
			Comment: "奥申，欧神",
		},
		"piggo": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://piggo.me/",
			Comment: "猪猪",
		},
		"ptchina": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://ptchina.org/",
			Comment: "铂金学院",
		},
		"ptsbao": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://ptsbao.club/",
			Comment: "烧包",
		},
		"pthome": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://pthome.net/",
			Comment: "铂金家",
		},
		"pttime": &config.SiteConfigStruct{
			Type:              "nexusphp",
			Aliases:           []string{"ptt"},
			Url:               "https://www.pttime.org/",
			TorrentsExtraUrls: []string{"https://www.pttime.org/adults.php"},
			Comment:           "PTT",
		},
		"sharkpt": &config.SiteConfigStruct{
			Type:                      "nexusphp",
			Url:                       "https://sharkpt.net/",
			SelectorTorrent:           ".torrent-action-bookmark",
			SelectorTorrentProcessBar: ".torrent-progress",
			SelectorUserInfo:          ".m_nav",
			Comment:                   "鲨鱼",
		},
		"soulvoice": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://pt.soulvoice.club/",
			Comment: "聆音",
		},
		"u2": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Aliases: []string{"dmhy"},
			Url:     "https://u2.dmhy.org/",
			Comment: "U2 (动漫花园)",
		},
		"ubits": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://ubits.club/",
			Comment: "你堡",
		},
		"ultrahd": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://ultrahd.net/",
			Comment: "UltraHD",
		},
		"uploads": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "http://uploads.ltd/",
			Comment: "Uploads 上传 | LTD 无限",
		},
		"wintersakura": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://wintersakura.net/",
			Comment: "冬樱",
		},
		"xinglin": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://xinglin.one/",
			Comment: "杏林",
		},
		"zmpt": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://zmpt.cc/",
			Comment: "织梦",
		},
	}
	SITENAMES = []string{}
)

func init() {
	for name := range SITES {
		SITENAMES = append(SITENAMES, name)
	}
	sort.Slice(SITENAMES, func(i, j int) bool {
		return SITENAMES[i] < SITENAMES[j]
	})
	for _, name := range SITENAMES {
		config := SITES[name]
		for _, alias := range config.Aliases {
			SITES[alias] = config
		}
		site.Register(&site.RegInfo{
			Name:    name,
			Aliases: config.Aliases,
			Creator: create,
		})
	}
}

func create(name string, siteConfig *config.SiteConfigStruct, globalConfig *config.ConfigStruct) (
	site.Site, error) {
	sc := *SITES[siteConfig.Type]           // copy
	utils.Assign(&sc, siteConfig, []int{0}) // field 0: type
	return site.CreateSiteInternal(name, &sc, globalConfig)
}

func FindSiteTypesByUrl(urlStr string) []string {
	urlObj, err := url.Parse(urlStr)
	if err != nil {
		return nil
	}
	return FindSiteTypesByHostname(urlObj.Hostname())
}

func FindSiteTypesByHostname(hostname string) []string {
	if hostname == "" {
		return nil
	}
	for sitename, site := range SITES {
		siteUrlObj, err := url.Parse(site.Url)
		if err != nil {
			continue
		}
		if hostname != siteUrlObj.Hostname() {
			continue
		}
		types := utils.CopySlice(site.Aliases)
		types = append(types, sitename)
		return types
	}
	return nil
}

func GuessSiteByHostname(hostname string, defaultSite string) string {
	// prefer defaultSite
	defaultSiteConfig := config.GeSiteConfig(defaultSite)
	if defaultSiteConfig != nil && defaultSiteConfig.Url != "" {
		urlObj, err := url.Parse(defaultSiteConfig.Url)
		if err == nil && urlObj.Hostname() == hostname {
			return defaultSite
		}
	}
	sitename := site.GetConfigSiteNameByHostname(hostname)
	if sitename != "" {
		return sitename
	}
	siteTypes := FindSiteTypesByHostname(hostname)
	if defaultSiteConfig != nil && slices.Index(siteTypes, defaultSiteConfig.Type) != -1 {
		return defaultSite
	}
	return site.GetConfigSiteNameByTypes(siteTypes...)
}
