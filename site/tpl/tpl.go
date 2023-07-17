package tpl

// 站点模板

import (
	"sort"

	"golang.org/x/exp/slices"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
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
			Type:      "nexusphp",
			Url:       "https://52pt.site/",
			GlobalHnR: true,
			Comment:   "52PT",
		},
		"audiences": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://audiences.me/",
			Domains: []string{"cinefiles.info"},
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
		"biho": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://www.biho.xyz/",
			Comment: "必火pt",
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
		"dajiao": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://dajiao.cyou/",
			Comment: "打胶",
		},
		"dhtclub": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://pt.dhtclub.com/",
			Comment: "DHTCLUB PT",
		},
		"dicmusic": &config.SiteConfigStruct{
			Type:    "gazelle",
			Url:     "https://dicmusic.club/",
			Comment: "海豚",
		},
		"discfan": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://discfan.net/",
			Comment: "蝶粉",
		},
		"eastgame": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Aliases: []string{"tlfbits", "tlf"},
			Url:     "https://pt.eastgame.org/",
			Comment: "吐鲁番",
		},
		"gainbound": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://gainbound.net/",
			Comment: "丐帮",
		},
		"gamegamept": &config.SiteConfigStruct{
			Type:              "nexusphp",
			Aliases:           []string{"ggpt"},
			GlobalHnR:         true,
			Url:               "https://www.gamegamept.com/",
			TorrentsExtraUrls: []string{"special.php"},
			Comment:           "GGPT",
		},
		"greatposterwall": &config.SiteConfigStruct{
			Type:    "gazellepw",
			Aliases: []string{"gpw"},
			Url:     "https://greatposterwall.com/",
			Comment: "海豹",
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
			SelectorTorrentSize:        `.video_size`,
			SelectorTorrentSeeders:     `.seeder_col`,
			SelectorTorrentLeechers:    `.leecher_col`,
			SelectorTorrentSnatched:    `.snatched_col`,
			Comment:                    "海胆",
		},
		"hares": &config.SiteConfigStruct{
			Type:                           "nexusphp",
			Aliases:                        []string{"haresclub"},
			Url:                            "https://club.hares.top/",
			SelectorUserInfoUploaded:       `li:has(i[title="上传量"])`,
			SelectorUserInfoDownloaded:     `li:has(i[title="下载量"])`,
			SelectorTorrentDiscountEndTime: `.layui-free-color span, .layui-twoupfree-color span`,
			Comment:                        "白兔俱乐部 (Hares Club)",
		},
		"hdarea": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://www.hdarea.co/",
			Domains: []string{"hdarea.club"},
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
			Domains:                    []string{"leniter.org"},
			SelectorTorrentDetailsLink: `a[href^="t-"]`,
			SelectorTorrentTime:        `.trtop > div:nth-last-child(2)@text`,
			SelectorTorrentSize:        `.trbo > div:nth-child(3)@text`,
			SelectorTorrentSeeders:     `a[title="种子数"] font`,
			SelectorTorrentLeechers:    ``,
			SelectorTorrentSnatched:    `a[title="完成数"]@text`,
			SelectorUserInfoUserName:   `#bottomnav a[href="userdetails"] strong`,
			SelectorUserInfoUploaded:   `#bottomnav a[href="userdetails"] i[title="上传量："]@after`,
			SelectorUserInfoDownloaded: `#bottomnav a[href="userdetails"] i[title="下载量："]@after`,
			UseCuhash:                  true,
			TorrentUrlIdRegexp:         `\bt-(?P<id>\d+)\b`,
			Comment:                    "城市",
		},
		"hdmayi": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "http://hdmayi.com/",
			Comment: "蚂蚁",
		},
		"hdpost": &config.SiteConfigStruct{
			Type:    "unit3d",
			Url:     "https://pt.hdpost.top/",
			Comment: "普斯特",
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
			Domains: []string{"hdfun.me"},
			Url:     "https://hdzone.me/",
			Comment: "高清地带",
		},
		"hhanclub": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Aliases: []string{"hh", "hhan"},
			Url:     "https://hhanclub.top/",
			Domains: []string{"hhan.club"},
			Comment: "憨憨",
		},
		"htpt": &config.SiteConfigStruct{
			Type:              "nexusphp",
			Url:               "https://www.htpt.cc/",
			TorrentsExtraUrls: []string{"live.php"},
			Comment:           "海棠",
		},
		"icc2022": &config.SiteConfigStruct{
			Type:      "nexusphp",
			Aliases:   []string{"icc"},
			Url:       "https://www.icc2022.com/",
			GlobalHnR: true,
			Comment:   "冰淇淋",
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
		"jpopsuki": &config.SiteConfigStruct{
			Type:                       "gazelle",
			Url:                        "https://jpopsuki.eu/",
			SelectorUserInfoUserName:   `#userinfo_username a.username`,
			SelectorUserInfoUploaded:   `#userinfo_stats li:nth-child(1)`,
			SelectorUserInfoDownloaded: `#userinfo_stats li:nth-child(2)`,
			Comment:                    "JPopsuki",
		},
		"jptvclub": &config.SiteConfigStruct{
			Type:                       "unit3d",
			Aliases:                    []string{"jptv"}, // Though there's another JPTVTS, I'd prefer the club.
			Url:                        "https://jptv.club/",
			Comment:                    "JPTV.club",
			SelectorUserInfoUploaded:   ".ratio-bar .badge-user:has(.fa-arrow-up)",
			SelectorUserInfoDownloaded: ".ratio-bar .badge-user:has(.fa-arrow-down)",
		},
		"kamept": &config.SiteConfigStruct{
			Type:              "nexusphp",
			Aliases:           []string{"kame"},
			Url:               "https://kamept.com/",
			TorrentsExtraUrls: []string{"special.php"}, // 龟龟的后花园
			Comment:           "KamePT",
		},
		"kufei": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://kufei.org/",
			Comment: "库非",
		},
		"leaves": &config.SiteConfigStruct{
			Type:                "nexusphp",
			Aliases:             []string{"redleaves"},
			Url:                 "https://leaves.red/",
			TorrentsExtraUrls:   []string{"special.php", "games.php"},
			SearchUrl:           `https://leaves.red/search.php?search=%s&search_area=0`,
			SelectorTorrentPaid: `span[title="收费种子"]`,
			Comment:             "红叶",
		},
		"lemonhd": &config.SiteConfigStruct{
			Type:                "nexusphp",
			Aliases:             []string{"leaguehd", "lemon"},
			Url:                 "https://lemonhd.org/",
			Domains:             []string{"leaguehd.com"},
			TorrentsUrl:         "https://lemonhd.org/torrents_new.php",
			SelectorTorrentFree: "div",
			Comment:             "柠檬",
		},
		"m-team": &config.SiteConfigStruct{
			Type:              "nexusphp",
			Aliases:           []string{"mteam", "mt"},
			Url:               "https://kp.m-team.cc/",
			TorrentsExtraUrls: []string{"adult.php", "music.php"},
			Comment:           "馒头",
		},
		"monikadesign": &config.SiteConfigStruct{
			Type:    "unit3d",
			Aliases: []string{"monika"},
			Url:     "https://monikadesign.uk/",
			Comment: "莫妮卡",
		},
		"nicept": &config.SiteConfigStruct{
			Type:      "nexusphp",
			Url:       "https://www.nicept.net/",
			GlobalHnR: true,
			Comment:   "老师",
		},
		"okpt": &config.SiteConfigStruct{
			Type:              "nexusphp",
			Url:               "https://www.okpt.net/",
			TorrentsExtraUrls: []string{"special.php"},
			Comment:           "OKPT",
		},
		"oldtoons": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Aliases: []string{"oldtoonsworld"},
			Url:     "https://oldtoons.world/",
			Comment: "Old Toons World",
		},
		"oshen": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://www.oshen.win/",
			Comment: "奥申，欧神",
		},
		"pandapt": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Aliases: []string{"panda"},
			Url:     "https://pandapt.net/",
			Comment: "熊猫高清",
		},
		"piggo": &config.SiteConfigStruct{
			Type:              "nexusphp",
			Url:               "https://piggo.me/",
			TorrentsExtraUrls: []string{"special.php"},
			Comment:           "猪猪",
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
			Type:                           "nexusphp",
			Aliases:                        []string{"ptt"},
			Url:                            "https://www.pttime.org/",
			TorrentsExtraUrls:              []string{"adults.php"},
			SelectorTorrentFree:            `.free`,
			SelectorTorrentDiscountEndTime: `.free + span`,
			Comment:                        "PTT",
		},
		"rousi": &config.SiteConfigStruct{
			Type:              "nexusphp",
			Url:               "https://rousi.zip/",
			TorrentsExtraUrls: []string{"special.php"},
			Comment:           "Rousi",
		},
		"sharkpt": &config.SiteConfigStruct{
			Type:                      "nexusphp",
			Url:                       "https://sharkpt.net/",
			SelectorTorrent:           ".torrent-action-bookmark",
			SelectorTorrentProcessBar: ".torrent-progress",
			SelectorUserInfo:          ".m_nav",
			SelectorTorrentFree:       ".s-tag",
			Comment:                   "鲨鱼",
		},
		"skyeysnow": &config.SiteConfigStruct{
			Type:    "discuz",
			Url:     "https://skyeysnow.com/",
			Domains: []string{"skyey.win", "skyey2.com"},
			Comment: "天雪",
		},
		"soulvoice": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Url:     "https://pt.soulvoice.club/",
			Comment: "聆音",
		},
		"tu88": &config.SiteConfigStruct{
			Type:              "nexusphp",
			Url:               "http://pt.tu88.men/",
			TorrentsExtraUrls: []string{"special.php"},
			GlobalHnR:         true,
			Comment:           "TU88",
		},
		"u2": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Aliases: []string{"dmhy"},
			Url:     "https://u2.dmhy.org/",
			Domains: []string{"dmhy.best"},
			// 下载中,下载完成过,做种中,当前未做种
			SelectorTorrentProcessBar: `.leechhlc_current,.snatchhlc_finish,.seedhlc_current,.seedhlc_ever_inenough`,
			Comment:                   "U2 (动漫花园)",
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
		"wukongwendao": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Aliases: []string{"wukong"},
			Url:     "https://wukongwendao.top/",
			Comment: "悟空问道",
		},
		"xingtan": &config.SiteConfigStruct{
			Type:    "nexusphp",
			Aliases: []string{"xinglin"},
			Url:     "https://xingtan.one/",
			Domains: []string{"xinglin.one"},
			Comment: "杏坛 (原杏林)",
		},
		"zhuque": &config.SiteConfigStruct{
			Type:    "tnode",
			Url:     "https://zhuque.in/",
			Comment: "朱雀",
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

func FindSiteTypesByDomain(domain string) []string {
	if domain == "" {
		return nil
	}
	for _, sitename := range SITENAMES {
		site := SITES[sitename]
		if !config.MatchSite(domain, site) {
			continue
		}
		types := utils.CopySlice(site.Aliases)
		types = append(types, sitename)
		return types
	}
	return nil
}

func GuessSiteByDomain(domain string, defaultSite string) string {
	// prefer defaultSite
	defaultSiteConfig := config.GetSiteConfig(defaultSite)
	if defaultSiteConfig != nil && config.MatchSite(domain, defaultSiteConfig) {
		return defaultSiteConfig.Name
	}
	sitename := site.GetConfigSiteNameByDomain(domain)
	if sitename != "" {
		return sitename
	}

	siteTypes := FindSiteTypesByDomain(domain)
	if defaultSiteConfig != nil && slices.Index(siteTypes, defaultSiteConfig.Type) != -1 {
		return defaultSite
	}
	return site.GetConfigSiteNameByTypes(siteTypes...)
}

func GuessSiteByTrackers(trackers []string, defaultSite string) string {
	for _, tracker := range trackers {
		domain := utils.GetUrlDomain(tracker)
		if domain == "" {
			continue
		}
		sitename := GuessSiteByDomain(domain, defaultSite)
		if sitename != "" {
			return sitename
		}
	}
	return ""
}
