package tpl

// 站点模板。
// CSS选择器使用 goquery 解析，支持 jQuery 的扩展语法(例如 :contains("txt") )。
// 除 Url 以外的所有 ***Url (例如 TorrentsUrl) 均应当使用相对路径。
// 大部分站点 id 使用其域名（去除二级域名和TLD后）的主要部分；部分站点域名与名称毫无关系，优先使用其通称。
// 站点 id 和 alias 长度限制在 15 个字符以内（最长："greatposterwall"），全小写并且不能包含任何特殊字符。

import (
	"slices"
	"sort"

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
			Dead:    true,
			Comment: "2xFree",
		},
		"3wmg": {
			Type:    "nexusphp",
			Url:     "https://www.3wmg.com/",
			Comment: "芒果",
			Dead:    true,
		},
		"52pt": {
			Type:      "nexusphp",
			Url:       "https://52pt.site/",
			GlobalHnR: true,
			Comment:   "52PT",
		},
		"aidoru-online": {
			Type:    "torrenttrader",
			Aliases: []string{"aidoruonline", "aidoru", "ao"},
			Domains: []string{"aidoru-online.org"},
			Url:     "https://aidoru-online.me/",
			Comment: "Aidoru!Online",
		},
		"audiences": {
			Type:                         "nexusphp",
			Aliases:                      []string{"ad"},
			Url:                          "https://audiences.me/",
			Domains:                      []string{"cinefiles.info"},
			SelectorTorrentCurrentActive: `.torrents-progress`,
			Comment:                      "观众",
		},
		"azusa": {
			Type:                    "nexusphp",
			Url:                     "https://azusa.wiki/",
			Domains:                 []string{"zimiao.icu"},
			SelectorTorrentSeeders:  `a[href$="seeders"]`,
			SelectorTorrentLeechers: `a[href$="leechers"]`,
			SelectorTorrentSnatched: `a[href^="viewsnatches"]`,
			Comment:                 "梓喵",
		},
		"beitai": {
			Type:    "nexusphp",
			Url:     "https://www.beitai.pt/",
			Dead:    true,
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
			Aliases:             []string{"byrbt"}, // reseed name
			Url:                 "https://byr.pt/",
			SelectorTorrentFree: `.pro_free, .pro_free2up`,
			Comment:             "北邮人",
			Insecure:            true,
		},
		"carpt": {
			Type:      "nexusphp",
			Url:       "https://carpt.net/",
			GlobalHnR: true,
			Comment:   "CarPT (小车站)",
		},
		"chdbits": {
			Type:               "nexusphp",
			Aliases:            []string{"ptchdbits", "rainbowisland", "chd"},
			Domains:            []string{"chdbits.xyz", "rainbowisland.co", "chdbits.co"},
			Url:                "https://ptchdbits.co/",
			SelectorTorrentHnR: `.circle-text:contains("h")`, // e.g. h5, h3, 表示 HnR 天数。
			Comment:            "彩虹岛",
		},
		"crabpt": {
			Type:              "nexusphp",
			Url:               "https://crabpt.vip/",
			TorrentsExtraUrls: []string{"special.php"},
			GlobalHnR:         true,
			Comment:           "蟹黄堡",
		},
		"cyanbug": {
			Type:    "nexusphp",
			Url:     "https://cyanbug.net/",
			Comment: "大青虫",
		},
		"dajiao": {
			Type:    "nexusphp",
			Url:     "https://dajiao.cyou/",
			Dead:    true,
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
		"ecust": {
			Type:              "nexusphp",
			Aliases:           []string{"ecustpt"},
			Domains:           []string{"ecustpt.eu.org"},
			Url:               "https://pt.ecust.pp.ua/",
			TorrentsExtraUrls: []string{"special.php"},
			Comment:           "ECUST PT (华东理工大学)",
		},
		// need heavy work to support it
		// "filelist": {
		// 	Type:              "nexusphp",
		// 	Url:               "https://filelist.io/",
		// 	TorrentsUrl:       "browse.php",
		// 	TorrentsExtraUrls: []string{"internal.php"},
		// },
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
			Aliases: []string{"ptgtk", "gtkpw"},
			Domains: []string{"pt.gtkpw.xyz"},
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
			Dead:                           true,
			Comment:                        "白兔俱乐部 (Hares Club)",
		},
		"hdarea": {
			Type:    "nexusphp",
			Url:     "https://hdarea.club/",
			Comment: "高清地带",
		},
		"hdatmos": {
			Type:    "nexusphp",
			Url:     "https://hdatmos.club/",
			Comment: "阿童木",
		},
		"hdclone": {
			Type:    "nexusphp",
			Url:     "https://pt.hdclone.org/",
			Comment: "HDClone",
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
		"hdkyl": {
			Type:    "nexusphp",
			Aliases: []string{"hdkylin"},
			Url:     "https://www.hdkyl.in/",
			Comment: "麒麟",
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
		"hdpt": {
			Type:    "nexusphp",
			Url:     "https://hdpt.xyz/",
			Comment: "明教",
		},
		"hdtime": {
			Type:    "nexusphp",
			Url:     "https://hdtime.org/",
			Comment: "高清时光",
		},
		"hdupt": {
			Type:    "nexusphp",
			Aliases: []string{"upxin", "hdu"},
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
			Dead:    true,
			Comment: "高清地带",
		},
		"hhanclub": {
			Type:                       "nexusphp",
			Aliases:                    []string{"hh", "hhan"},
			Url:                        "https://hhanclub.top/",
			Domains:                    []string{"hhan.club"},
			SelectorUserInfoUploaded:   `img[alt="上传"],img[alt="上傳"]@after`,
			SelectorUserInfoDownloaded: `img[alt="下载"],img[alt="下載"]@after`,
			SelectorTorrentSize:        `.torrent-info-text-size`,
			SelectorTorrentTime:        `.torrent-info-text-added`,
			SelectorTorrentSeeders:     `.torrent-info-text-seeders`,
			SelectorTorrentLeechers:    `.torrent-info-text-leechers`,
			SelectorTorrentSnatched:    `.torrent-info-text-finished`,
			SelectorTorrentActive:      `.w-full[title$="%"] .h-full`,
			Comment:                    "憨憨",
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
			Type:    "nexusphp",
			Aliases: []string{"icc"},
			Url:     "https://www.icc2022.com/",
			Comment: "冰淇淋",
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
			// 龟龟图床 https://pic.kamept.com/ .
			// 返回的图片 URL: https://p.kamept.com/ffffff-Full.webp .
			// 有 Referer 限制，仅限 kamept 站内引用（直接打开图片也会 403）。
			ImageUploadUrl:              "https://pic.kamept.com/upload/k",
			ImageUploadFileField:        "file",
			ImageUploadResponseUrlField: "url",
			ImageUploadHeaders: [][]string{
				{"Cookie", "c_secure_uid=dummy; c_secure_pass=dummy; c_secure_ssl=dummy; c_secure_tracker_ssl=dummy; c_secure_login=dummy; search_box=minus"},
			},
			UploadTorrentPayloadRequiredKeys: "type",
			UploadTorrentAdditionalPayload: map[string]string{
				"descr": `
{% if _cover %}
[img]{{_cover}}[/img]
{% endif %}
{% if _images %}
{% for image in _images %}
[img]{{image}}[/img]
{% endfor %}
{% endif %}
{% if number | regex_search("\\bRJ\\d{5,12}\\b") %}
[dlsite]{{number | regex_search("\\bRJ\\d{5,12}\\b")}}[/dlsite]
https://www.dlsite.com/maniax/work/=/product_id/{{number | regex_search("\\bRJ\\d{5,12}\\b")}}.html
{% endif %}
{% if number | regex_search("\\bd_\\d{5,12}\\b") %}
https://www.dmm.co.jp/dc/doujin/-/detail/=/cid={{number | regex_search("\\bd_\\d{5,12}\\b")}}/
{% endif %}
{% if _meta %}
{{_meta}}
{% endif %}
{{_text}}{% if comment %}

---

{{comment}}{% endif %}`,
				// 420: 外语音声; 415: 游戏; 435: 同人志; 411: 2D动画; 423: 3D动画.
				// dlsite work_type genres: https://www.dlsite.com/maniax/worktype/list .
				"type": `
{%- if tags -%}
	{%- if "ボイス・ASMR" in tags -%}420
	{%- elif
		"アクション" in tags or
		"クイズ" in tags or
		"アドベンチャー" in tags or
		"ロールプレイング" in tags or
		"テーブル" in tags or
		"デジタルノベル" in tags or
		"シミュレーション" in tags or
		"タイピング" in tags or
		"シューティング" in tags or
		"パズル" in tags or
		"その他ゲーム" in tags
	-%}415
	{%- elif "動画" in tags -%}
		{%- if "3D作品" in tags -%}423
		{%- else -%}411
		{%- endif -%}
	{%- elif "マンガ" in tags -%}435
	{%- elif "音声" in tags or "音声あり" in tags -%}420
	{%- endif -%}
{%- endif -%}
`,
			},
			Comment: "KamePT",
		},
		"keepfrds": {
			Type:                   "nexusphp",
			Aliases:                []string{"frds"},
			Url:                    "https://pt.keepfrds.com/",
			SelectorTorrentActive:  `div[style="width: 400px; height:16px;"] > div[style="margin-top: 2px; float: left;"]`,
			SelectorTorrentNeutral: `img.pro_nl`,
			Comment:                "朋友、月月",
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
		// old lemonhd
		// "lemonhd": {
		// 	Type:                "nexusphp",
		// 	Aliases:             []string{"leaguehd", "lemon"},
		// 	Url:                 "https://lemonhd.org/",
		// 	Domains:             []string{"leaguehd.com"},
		// 	TorrentsUrl:         "torrents_new.php",
		// 	SelectorTorrentFree: `div:contains("免費")`,
		// 	Comment:             "柠檬",
		// 	Dead:                true,
		// },
		"lemonhd": {
			Type:    "nexusphp",
			Aliases: []string{"leaguehd", "lemon"},
			Url:     "https://lemonhd.club/",
			Comment: "柠檬(新)",
		},
		"mteam": {
			Type:                "mtorrent",
			Aliases:             []string{"m-team", "mt"},
			Url:                 "https://api.m-team.cc/", // @todo: add a separated ApiUrl (or similar) field.
			Domains:             []string{"m-team.io"},
			NoCookie:            true, // @todo: add a "Token" field for API authentication.
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
			Type:    "unit3d",
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
		"pterclub": {
			Type:    "nexusphp",
			Aliases: []string{"pter"},
			Url:     "https://pterclub.com/",
			Comment: "猫站",
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
		"ptlsp": {
			Type:    "nexusphp",
			Url:     "https://www.ptlsp.com/",
			Dead:    true,
			Comment: "",
		},
		"pttime": {
			Type:                           "nexusphp",
			Aliases:                        []string{"ptt"},
			Url:                            "https://www.pttime.org/",
			TorrentsExtraUrls:              []string{"adults.php"},
			SelectorTorrentFree:            `.free`,
			SelectorTorrentNoTraffic:       `.zeroupzerodown`,
			SelectorTorrentDiscountEndTime: `.free + span`,
			UsePasskey:                     true,
			Comment:                        "PTT",
		},
		"ptvicomo": {
			Type:                     "nexusphp",
			Url:                      "https://ptvicomo.net/",
			TorrentsExtraUrls:        []string{"special.php"},
			SelectorUserInfoUserName: `.User_Name`,
			Comment:                  "象站",
		},
		"ptzone": {
			Type:    "nexusphp",
			Url:     "https://ptzone.xyz/",
			Comment: "PTzone",
		},
		"pwtorrents": {
			Type:                       "nexusphp",
			Url:                        "https://pwtorrents.net/",
			TorrentsUrl:                "browse.php",
			SelectorUserInfoUploaded:   `img[alt="Uploaded amount"]+font`,
			SelectorUserInfoDownloaded: `img[alt="Downloaded amount"]+font`,
			SelectorTorrentFree:        `img[src="pic/freeleech.png"]`,
			AcceptAnyHttpStatus:        true, // 该站点种子页面 http status == 500
			GlobalHnR:                  true,
			Timezone:                   "UTC",
			Comment:                    "Pro Wrestling Torrents",
		},
		"qingwapt": {
			Type:    "nexusphp",
			Aliases: []string{"qingwa"},
			Url:     "https://www.qingwapt.com/",
			Comment: "青蛙",
		},
		"raingfh": {
			Type:    "nexusphp",
			Url:     "https://raingfh.top/",
			Comment: "雨",
		},
		"rousi": {
			Type:              "nexusphp",
			Url:               "https://rousi.zip/",
			TorrentsExtraUrls: []string{"special.php"},
			Comment:           "Rousi",
		},
		"sharkpt": {
			Type:                   "nexusphp",
			Url:                    "https://sharkpt.net/",
			SelectorTorrent:        ".torrent-action-bookmark",
			SelectorTorrentActive:  ".torrent-progress",
			SelectorUserInfo:       ".m_nav",
			SelectorTorrentFree:    `.s-tag:contains("FREE")`,
			SelectorTorrentNeutral: `.s-tag-neutral`,
			Comment:                "鲨鱼",
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
		"tccf": {
			Type:    "nexusphp",
			Aliases: []string{"et8", "torrentccf"},
			Url:     "https://et8.org/",
			Comment: "精品论坛，他吹吹风",
		},
		"tlfbits": {
			Type:              "nexusphp",
			Aliases:           []string{"eastgame", "tlf"},
			Url:               "https://pt.eastgame.org/",
			TorrentsExtraUrls: []string{"trls.php"},
			Comment:           "吐鲁番",
		},
		"tosky": {
			Type:    "nexusphp",
			Url:     "https://t.tosky.club/",
			Comment: "ToSky",
		},
		"totheglory": {
			Type:              "nexusphp",
			Aliases:           []string{"ttg"},
			Url:               "https://totheglory.im/",
			TorrentsUrl:       "browse.php?c=M",
			TorrentsExtraUrls: []string{"browse.php?c=G"},
			// 是否能够直接同时搜索影视区和游戏区？
			// SearchUrl:                   "browse.php?search_field=%s",
			SearchQueryVariable:         "search_field",
			SelectorTorrent:             `a.dl_a[href^="/dl/"]`,
			SelectorTorrentDownloadLink: `a.dl_a[href^="/dl/"]`,
			SelectorTorrentDetailsLink:  `.name_left a[href^="/t/"]`,
			SelectorTorrentSeeders:      `a[href$="&toseeders=1"]`,
			SelectorTorrentLeechers:     `a[href$="&todlers=1"]`,
			SelectorTorrentSnatched:     `td:nth-child(8)@text`, // it's ugly
			SelectorTorrentActive:       `.process`,
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
			// 下载中,下载完成过,做种中,当前未做种: .leechhlc_current,.snatchhlc_finish,.seedhlc_current,.seedhlc_ever_inenough
			SelectorTorrentActive:        `.leechhlc_current,.seedhlc_current`,
			SelectorTorrentCurrentActive: `.snatchhlc_finish,.seedhlc_ever_inenough`,
			SelectorTorrentFree:          `img.arrowdown + b:contains("0.00X")`,
			Comment:                      "U2 (动漫花园)",
		},
		"ubits": {
			Type:    "nexusphp",
			Aliases: []string{"ub"},
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
			Dead:    true,
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
	SITENAMES = []string{} // all internal site (canonical) names in lexical order.
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
	if defaultSiteConfig != nil && slices.Contains(siteTypes, defaultSiteConfig.Type) {
		return defaultSite, nil
	}
	return site.GetConfigSiteNameByTypes(siteTypes...)
}

// Try to match trackers of a torrent with a site in ptool.toml config file.
// If trackers do NOT match any site in ptool.toml, return "",nil;
// if trackers match with multiple sites in ptool.toml, return "" and an "ambiguous result" error.
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
