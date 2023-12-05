package tpl

// 站点模板。
// CSS选择器使用 goquery 解析，支持 jQuery 的扩展语法(例如 :contains("txt") )。
// 除 Url 以外的所有 ***Url (例如 TorrentsUrl) 均应当使用相对路径

import (
	"sort"

	"golang.org/x/exp/slices"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
)

var (
	SITES = map[string]*config.SiteConfigStruct{
		"0ff": {
			Type:    "nexusphp",
			Aliases: []string{"pt0ffcc"},
			Url:     "https://pt.0ff.cc/",
			Comment: "自由农场",
		},
		"1ptba": {
			Type:    "nexusphp",
			Url:     "https://1ptba.com/",
			Comment: "1PTA (壹PT吧)",
		},
		"2xfree": {
			Type:    "nexusphp",
			Aliases: []string{"pt2xfree"},
			Url:     "https://pt.2xfree.org/",
			Comment: "2xFree",
		},
		"3wmg": {
			Type:    "nexusphp",
			Url:     "https://www.3wmg.com/",
			Comment: "芒果",
		},
		"52pt": {
			Type:      "nexusphp",
			Url:       "https://52pt.site/",
			GlobalHnR: true,
			Comment:   "52PT",
		},
		"audiences": {
			Type:    "nexusphp",
			Aliases: []string{"ad"},
			Url:     "https://audiences.me/",
			Domains: []string{"cinefiles.info"},
			Comment: "观众",
		},
		"azusa": {
			Type:    "nexusphp",
			Url:     "https://azusa.wiki/",
			Comment: "梓喵",
		},
		"beitai": {
			Type:    "nexusphp",
			Url:     "https://www.beitai.pt/",
			Comment: "备胎",
		},
		"biho": {
			Type:    "nexusphp",
			Url:     "https://www.biho.xyz/",
			Comment: "必火pt",
		},
		"btschool": {
			Type:      "nexusphp",
			Url:       "https://pt.btschool.club/",
			GlobalHnR: true,
			Comment:   "学校",
		},
		"byr": {
			Type:                "nexusphp",
			Url:                 "https://byr.pt/",
			SelectorTorrentFree: `.pro_free, .pro_free2up`,
			Comment:             "北邮人",
		},
		"carpt": {
			Type:      "nexusphp",
			Url:       "https://carpt.net/",
			GlobalHnR: true,
			Comment:   "CarPT (小车站)",
		},
		"cyanbug": {
			Type:    "nexusphp",
			Url:     "https://cyanbug.net/",
			Comment: "大青虫",
		},
		"dajiao": {
			Type:    "nexusphp",
			Url:     "https://dajiao.cyou/",
			Comment: "打胶",
		},
		"dhtclub": {
			Type:    "nexusphp",
			Url:     "https://pt.dhtclub.com/",
			Comment: "DHTCLUB PT",
		},
		"dicmusic": {
			Type:    "gazelle",
			Aliases: []string{"dic"},
			Domains: []string{"dicmusic.club", "52dic.vip"},
			Url:     "https://dicmusic.com/",
			Comment: "海豚",
		},
		"discfan": {
			Type:    "nexusphp",
			Url:     "https://discfan.net/",
			Comment: "蝶粉",
		},
		"dragonhd": {
			Type:    "nexusphp",
			Url:     "https://www.dragonhd.xyz/",
			Comment: "龍之家",
		},
		"eastgame": {
			Type:    "nexusphp",
			Aliases: []string{"tlfbits", "tlf"},
			Url:     "https://pt.eastgame.org/",
			Comment: "吐鲁番",
		},
		"gainbound": {
			Type:    "nexusphp",
			Url:     "https://gainbound.net/",
			Comment: "丐帮",
		},
		"gamegamept": {
			Type:              "nexusphp",
			Aliases:           []string{"ggpt"},
			GlobalHnR:         true,
			Url:               "https://www.gamegamept.com/",
			TorrentsExtraUrls: []string{"special.php"},
			Comment:           "GGPT",
		},
		"gamerapt": {
			Type:    "nexusphp",
			Url:     "https://gamerapt.link/",
			Comment: "駕瞑羅",
		},
		"greatposterwall": {
			Type:    "gazellepw",
			Aliases: []string{"gpw"},
			Url:     "https://greatposterwall.com/",
			Comment: "海豹",
		},
		"gtk": {
			Type:    "nexusphp",
			Aliases: []string{"ptgtk"},
			Url:     "https://pt.gtk.pw/",
			Comment: "PT GTK",
		},
		"haidan": {
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
		"hares": {
			Type:                           "nexusphp",
			Aliases:                        []string{"haresclub"},
			Url:                            "https://club.hares.top/",
			SelectorUserInfoUploaded:       `li:has(i[title="上传量"])`,
			SelectorUserInfoDownloaded:     `li:has(i[title="下载量"])`,
			SelectorTorrentDiscountEndTime: `.layui-free-color span, .layui-twoupfree-color span`,
			Comment:                        "白兔俱乐部 (Hares Club)",
		},
		"hdarea": {
			Type:    "nexusphp",
			Url:     "https://www.hdarea.co/",
			Domains: []string{"hdarea.club"},
			Comment: "高清地带",
		},
		"hdatmos": {
			Type:    "nexusphp",
			Url:     "https://hdatmos.club/",
			Comment: "阿童木",
		},
		"hddolby": {
			Type:    "nexusphp",
			Url:     "https://www.hddolby.com/",
			Comment: "杜比",
		},
		"hdfans": {
			Type:    "nexusphp",
			Url:     "https://hdfans.org/",
			Comment: "红豆饭",
		},
		"hdhome": {
			Type:    "nexusphp",
			Url:     "https://hdhome.org/",
			Comment: "家园",
		},
		"hdcity": {
			Type:                       "nexusphp",
			Url:                        "https://hdcity.city/",
			SearchUrl:                  "pt?iwannaseethis=%s",
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
			TorrentDownloadUrl:         `download?id={id}`,
			TorrentUrlIdRegexp:         `\bt-(?P<id>\d+)\b`,
			Comment:                    "城市",
		},
		"hdmayi": {
			Type:    "nexusphp",
			Url:     "http://hdmayi.com/",
			Comment: "蚂蚁",
		},
		"hdpost": {
			Type:    "unit3d",
			Url:     "https://pt.hdpost.top/",
			Comment: "普斯特",
		},
		"hdtime": {
			Type:    "nexusphp",
			Url:     "https://hdtime.org/",
			Comment: "高清时光",
		},
		"hdupt": {
			Type:    "nexusphp",
			Aliases: []string{"upxin"},
			Url:     "https://pt.hdupt.com/",
			Comment: "好多油",
		},
		"hdvideo": {
			Type:    "nexusphp",
			Url:     "https://hdvideo.one/",
			Comment: "高清视频",
		},
		"hdzone": {
			Type:    "nexusphp",
			Domains: []string{"hdfun.me"},
			Url:     "https://hdzone.me/",
			Comment: "高清地带",
		},
		"hhanclub": {
			Type:    "nexusphp",
			Aliases: []string{"hh", "hhan"},
			Url:     "https://hhanclub.top/",
			Domains: []string{"hhan.club"},
			Comment: "憨憨",
		},
		"htpt": {
			Type:              "nexusphp",
			Url:               "https://www.htpt.cc/",
			TorrentsExtraUrls: []string{"live.php"},
			Comment:           "海棠",
		},
		"hudbt": {
			Type:                       "nexusphp",
			Url:                        "https://hudbt.hust.edu.cn/",
			Insecure:                   true, // 该站点的 TLS 证书有问题 (2023-11 测试)
			SelectorTorrent:            `a[href*="/download.php?id="]`,
			SelectorTorrentDetailsLink: `a[href*="/details.php?id="]`,
			SelectorTorrentFree:        `img.free, img.twoupfree`,
			Comment:                    "蝴蝶",
		},
		"icc2022": {
			Type:      "nexusphp",
			Aliases:   []string{"icc"},
			Url:       "https://www.icc2022.com/",
			GlobalHnR: true,
			Comment:   "冰淇淋",
		},
		"ilolicon": {
			Type:    "nexusphp",
			Url:     "https://share.ilolicon.com/",
			Comment: "ilolicon PT",
		},
		"itzmx": {
			Type:    "nexusphp",
			Url:     "https://pt.itzmx.com/",
			Comment: "PT分享站",
		},
		"joyhd": {
			Type:    "nexusphp",
			Url:     "https://www.joyhd.net/",
			Comment: "JoyHD",
		},
		"jpopsuki": {
			Type:                       "gazelle",
			Url:                        "https://jpopsuki.eu/",
			SelectorUserInfoUserName:   `#userinfo_username a.username`,
			SelectorUserInfoUploaded:   `#userinfo_stats li:nth-child(1)`,
			SelectorUserInfoDownloaded: `#userinfo_stats li:nth-child(2)`,
			Comment:                    "JPopsuki",
		},
		"jptvclub": {
			Type:                       "unit3d",
			Aliases:                    []string{"jptv"}, // Though there's another JPTVTS, I'd prefer the club.
			Url:                        "https://jptv.club/",
			Comment:                    "JPTV.club",
			SelectorUserInfoUploaded:   ".ratio-bar .badge-user:has(.fa-arrow-up)",
			SelectorUserInfoDownloaded: ".ratio-bar .badge-user:has(.fa-arrow-down)",
		},
		"kamept": {
			Type:              "nexusphp",
			Aliases:           []string{"kame"},
			Url:               "https://kamept.com/",
			TorrentsExtraUrls: []string{"special.php"}, // 龟龟的后花园
			Comment:           "KamePT",
		},
		"keepfrds": {
			Type:                      "nexusphp",
			Aliases:                   []string{"frds"},
			Url:                       "https://pt.keepfrds.com/",
			SelectorTorrentProcessBar: `div[style="width: 400px; height:16px;"] > div[style="margin-top: 2px; float: left;"]`,
			SelectorTorrentNeutral:    `img.pro_nl`,
			Comment:                   "朋友、月月",
		},
		"kufei": {
			Type:    "nexusphp",
			Url:     "https://kufei.org/",
			Comment: "库非",
		},
		"leaves": {
			Type:              "nexusphp",
			Aliases:           []string{"redleaves"},
			Url:               "https://leaves.red/",
			TorrentsExtraUrls: []string{"special.php", "games.php"},
			SearchUrl:         `search.php?search=%s&search_area=0`,
			Comment:           "红叶",
		},
		"lemonhd": {
			Type:                "nexusphp",
			Aliases:             []string{"leaguehd", "lemon"},
			Url:                 "https://lemonhd.org/",
			Domains:             []string{"leaguehd.com"},
			TorrentsUrl:         "torrents_new.php",
			SelectorTorrentFree: `div:contains("免費")`,
			Comment:             "柠檬",
		},
		"m-team": {
			Type:                "nexusphp",
			Aliases:             []string{"mteam", "mt"},
			Url:                 "https://kp.m-team.cc/",
			Domains:             []string{"m-team.io"},
			TorrentsExtraUrls:   []string{"adult.php", "music.php"},
			BrushExcludes:       []string{"[原盤首發]"}, // 馒头原盘首发限速，刷流效果极差
			FlowControlInterval: 30,                 // 馒头流控极为严格。很容易出“休息120秒”页面
			Comment:             "馒头",
		},
		"monikadesign": {
			Type:    "unit3d",
			Aliases: []string{"monika"},
			Url:     "https://monikadesign.uk/",
			Comment: "莫妮卡",
		},
		"nicept": {
			Type:      "nexusphp",
			Url:       "https://www.nicept.net/",
			GlobalHnR: true,
			Comment:   "老师",
		},
		"okpt": {
			Type:              "nexusphp",
			Url:               "https://www.okpt.net/",
			TorrentsExtraUrls: []string{"special.php"},
			Comment:           "OKPT",
		},
		"oldtoons": {
			Type:    "nexusphp",
			Aliases: []string{"oldtoonsworld"},
			Url:     "https://oldtoons.world/",
			Comment: "Old Toons World",
		},
		"oshen": {
			Type:    "nexusphp",
			Url:     "https://www.oshen.win/",
			Comment: "奥申，欧神",
		},
		"ourbits": {
			Type:    "nexusphp",
			Aliases: []string{"ob"},
			Url:     "https://ourbits.club/",
			Comment: "我堡",
		},
		"pandapt": {
			Type:    "nexusphp",
			Aliases: []string{"panda"},
			Url:     "https://pandapt.net/",
			Comment: "熊猫高清",
		},
		"piggo": {
			Type:              "nexusphp",
			Url:               "https://piggo.me/",
			TorrentsExtraUrls: []string{"special.php"},
			SearchUrl:         `search.php?search=%s&search_area=0`,
			Comment:           "猪猪",
		},
		"ptcafe": {
			Type:    "nexusphp",
			Url:     "https://ptcafe.club/",
			Comment: "咖啡",
		},
		"ptchina": {
			Type:    "nexusphp",
			Url:     "https://ptchina.org/",
			Comment: "铂金学院",
		},
		"ptsbao": {
			Type:    "nexusphp",
			Url:     "https://ptsbao.club/",
			Comment: "烧包",
		},
		"pthome": {
			Type:    "nexusphp",
			Url:     "https://pthome.net/",
			Comment: "铂金家",
		},
		"pttime": {
			Type:                           "nexusphp",
			Aliases:                        []string{"ptt"},
			Url:                            "https://www.pttime.org/",
			TorrentsExtraUrls:              []string{"adults.php"},
			SelectorTorrentFree:            `.free`,
			SelectorTorrentNoTraffic:       `.zeroupzerodown`,
			SelectorTorrentDiscountEndTime: `.free + span`,
			Comment:                        "PTT",
		},
		"rousi": {
			Type:              "nexusphp",
			Url:               "https://rousi.zip/",
			TorrentsExtraUrls: []string{"special.php"},
			Comment:           "Rousi",
		},
		"sharkpt": {
			Type:                      "nexusphp",
			Url:                       "https://sharkpt.net/",
			SelectorTorrent:           ".torrent-action-bookmark",
			SelectorTorrentProcessBar: ".torrent-progress",
			SelectorUserInfo:          ".m_nav",
			SelectorTorrentFree:       `.s-tag:contains("FREE")`,
			SelectorTorrentNeutral:    `.s-tag-neutral`,
			Comment:                   "鲨鱼",
		},
		"skyeysnow": {
			Type:    "discuz",
			Url:     "https://skyeysnow.com/",
			Domains: []string{"skyey.win", "skyey2.com"},
			Comment: "天雪",
		},
		"soulvoice": {
			Type:    "nexusphp",
			Url:     "https://pt.soulvoice.club/",
			Comment: "聆音",
		},
		"totheglory": {
			Type:                        "nexusphp",
			Aliases:                     []string{"ttg"},
			Url:                         "https://totheglory.im/",
			TorrentsUrl:                 "browse.php?c=M",
			TorrentsExtraUrls:           []string{"browse.php?c=G"},
			SelectorTorrent:             `a.dl_a[href^="/dl/"]`,
			SelectorTorrentDownloadLink: `a.dl_a[href^="/dl/"]`,
			SelectorTorrentDetailsLink:  `.name_left a[href^="/t/"]`,
			SelectorTorrentSeeders:      `a[href$="&toseeders=1"]`,
			SelectorTorrentLeechers:     `a[href$="&todlers=1"]`,
			SelectorTorrentSnatched:     `td:nth-child(8)@text`, // it's ugly
			SelectorTorrentProcessBar:   `.process`,
			TorrentDownloadUrl:          `dl/{id}`,
			TorrentDownloadUrlPrefix:    `dl/`,
			TorrentUrlIdRegexp:          `\b(t|dl)/(?P<id>\d+)\b`,
			UseDigitHash:                true,
			NexusphpNoLetDown:           true,
			Comment:                     "听听歌、套",
		},
		"tu88": {
			Type:              "nexusphp",
			Url:               "http://pt.tu88.men/",
			TorrentsExtraUrls: []string{"special.php"},
			GlobalHnR:         true,
			Comment:           "TU88",
		},
		"u2": {
			Type:    "nexusphp",
			Aliases: []string{"dmhy"},
			Url:     "https://u2.dmhy.org/",
			Domains: []string{"dmhy.best"},
			// 下载中,下载完成过,做种中,当前未做种
			SelectorTorrentProcessBar: `.leechhlc_current,.snatchhlc_finish,.seedhlc_current,.seedhlc_ever_inenough`,
			SelectorTorrentFree:       `img.arrowdown + b:contains("0.00X")`,
			Comment:                   "U2 (动漫花园)",
		},
		"ubits": {
			Type:    "nexusphp",
			Url:     "https://ubits.club/",
			Comment: "你堡",
		},
		"ultrahd": {
			Type:    "nexusphp",
			Url:     "https://ultrahd.net/",
			Comment: "UltraHD",
		},
		"uploads": {
			Type:    "nexusphp",
			Url:     "http://uploads.ltd/",
			Comment: "Uploads 上传 | LTD 无限",
		},
		"wintersakura": {
			Type:    "nexusphp",
			Aliases: []string{"wtsakura"},
			Url:     "https://wintersakura.net/",
			Comment: "冬樱",
		},
		"wukongwendao": {
			Type:    "nexusphp",
			Aliases: []string{"wukong"},
			Url:     "https://wukongwendao.top/",
			Comment: "悟空问道",
		},
		"xingtan": {
			Type:    "nexusphp",
			Aliases: []string{"xinglin"},
			Url:     "https://xingtan.one/",
			Domains: []string{"xinglin.one"},
			Comment: "杏坛 (原杏林)",
		},
		"zhuque": {
			Type:    "tnode",
			Url:     "https://zhuque.in/",
			Comment: "朱雀",
		},
		"zmpt": {
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
	sc := *SITES[siteConfig.Type]          // copy
	util.Assign(&sc, siteConfig, []int{0}) // field 0: type
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
		types := util.CopySlice(site.Aliases)
		types = append(types, sitename)
		return types
	}
	return nil
}

func GuessSiteByDomain(domain string, defaultSite string) (string, error) {
	// prefer defaultSite
	defaultSiteConfig := config.GetSiteConfig(defaultSite)
	if defaultSiteConfig != nil && config.MatchSite(domain, defaultSiteConfig) {
		return defaultSiteConfig.GetName(), nil
	}
	sitename, err := site.GetConfigSiteNameByDomain(domain)
	if sitename != "" || err != nil {
		return sitename, err
	}

	siteTypes := FindSiteTypesByDomain(domain)
	if defaultSiteConfig != nil && slices.Index(siteTypes, defaultSiteConfig.Type) != -1 {
		return defaultSite, nil
	}
	return site.GetConfigSiteNameByTypes(siteTypes...)
}

func GuessSiteByTrackers(trackers []string, defaultSite string) (string, error) {
	for _, tracker := range trackers {
		domain := util.GetUrlDomain(tracker)
		if domain == "" {
			continue
		}
		sitename, err := GuessSiteByDomain(domain, defaultSite)
		if sitename != "" || err != nil {
			return sitename, err
		}
	}
	return "", nil
}
