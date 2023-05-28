package nexusphp

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html"

	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
)

const (
	SELECTOR_TORRENT               = `a[href^="download.php?"],a[href^="download?"]`
	SELECTOR_DOWNLOAD_LINK         = `a[href^="download.php?"],a[href^="download?"]`
	SELECTOR_DETAILS_LINK          = `a[href^="details.php?"],a[href^="details_"]`
	SELECTOR_TORRENTS_LIST_DEFAULT = `table.torrents > tbody`
)

type TorrentsParserOption struct {
	location                    *time.Location
	siteurl                     string
	selectorTorrentsListHeader  string
	selectorTorrentsList        string
	selectorTorrentBlock        string
	selectorTorrent             string
	selectorTorrentDownloadLink string
	selectorTorrentDetailsLink  string
	selectorTorrentTime         string
	SelectorTorrentSeeders      string
	SelectorTorrentLeechers     string
	SelectorTorrentSnatched     string
	SelectorTorrentSize         string
	SelectorTorrentProcessBar   string
	SelectorTorrentFree         string
}

func parseTorrents(doc *goquery.Document, option *TorrentsParserOption,
	doctime int64, siteName string) (torrents []site.Torrent, err error) {
	torrents = []site.Torrent{}
	if option.selectorTorrent == "" {
		option.selectorTorrent = SELECTOR_TORRENT
	}
	if option.selectorTorrentDownloadLink == "" {
		option.selectorTorrentDownloadLink = SELECTOR_DOWNLOAD_LINK
	}
	if option.selectorTorrentDetailsLink == "" {
		option.selectorTorrentDetailsLink = SELECTOR_DETAILS_LINK
	}

	// @todo: improve this fragile checking
	globalDiscountEndTime := int64(0)
	doc.Find(`b a[href="torrents.php"]`).EachWithBreak(func(i int, s *goquery.Selection) bool {
		txt := utils.DomSanitizedText(s)
		// eg. '全站 [2X Free] 生效中！时间：2023-04-12 13:50:00 ~ 2023-04-15 13:50:00'
		re := regexp.MustCompile(`全站 \[(?P<discount>(\dX Free|Free))\] 生效中！时间：(?P<start>[-\d\s:]+)\s*~\s*(?P<end>[-\d\s:]+)`)
		m := re.FindStringSubmatch(txt)
		if m != nil {
			t1, err1 := utils.ParseTime(m[re.SubexpIndex("start")], option.location)
			t2, err2 := utils.ParseTime(m[re.SubexpIndex("end")], option.location)
			if err1 == nil && err2 == nil && doctime >= t1 && doctime < t2 {
				globalDiscountEndTime = t2
			}
			return false
		}
		return true
	})

	var containerElNode *html.Node
	var containerEl *goquery.Selection
	torrentEls := doc.Find(option.selectorTorrent)
	if option.selectorTorrentsList != "" {
		containerEl = doc.Find(option.selectorTorrentsList)
		containerElNode = containerEl.Get(0)
	} else if torrentEls.Length() > 1 {
		commonParentsCnt := map[*html.Node](int64){}
		nodeSelectionMap := map[*html.Node](*goquery.Selection){}
		var previousEl *goquery.Selection = nil
		torrentEls.Each(func(i int, el *goquery.Selection) {
			if i == 0 {
				previousEl = el
				return
			}
			commonParent := domCommonParent(previousEl, el)
			if commonParent != nil {
				commonParentsCnt[commonParent.Get(0)] += 1
				nodeSelectionMap[commonParent.Get(0)] = commonParent
			}
			previousEl = el
		})
		log.Tracef("nptr: map elsLen=%d, len=%d\n", torrentEls.Length(), len(commonParentsCnt))
		containerElNode = utils.MapMaxElementKey(commonParentsCnt)
		if containerElNode != nil {
			containerEl = nodeSelectionMap[containerElNode]
		}
	} else if torrentEls.Length() == 1 { // only one torrent found (eg. search result page)
		containerEl = doc.Find(SELECTOR_TORRENTS_LIST_DEFAULT)
		if containerEl.Length() > 0 {
			containerElNode = containerEl.Get(0)
		} else {
			parent := torrentEls.Parent()
			for parent.Length() > 0 {
				if parent.Children().Length() == 1 {
					break
				}
				parent = parent.Parent()
			}
			if parent.Length() > 0 && parent.Children().Length() == 1 {
				containerEl = parent
				containerElNode = parent.Get(0)
			}
		}
	}
	if containerElNode == nil {
		if torrentEls.Length() > 1 || option.selectorTorrentsList != "" {
			err = fmt.Errorf("cann't find torrents list container element")
		} else {
			log.Tracef("Cann't find torrents list container element.")
		}
		return
	}
	log.Tracef("nptr: container node=%v, id=%v, class=%v\n",
		containerElNode,
		containerEl.AttrOr("id", ""),
		containerEl.AttrOr("class", ""),
	)

	fieldColumIndex := map[string]int{
		"time":     -1,
		"size":     -1,
		"seeders":  -1,
		"leechers": -1,
		"snatched": -1,
		"title":    -1,
		"process":  -1,
	}
	var headerEl *goquery.Selection
	if option.selectorTorrentsListHeader != "" {
		headerEl = containerEl.Find(option.selectorTorrentsListHeader).First()
	} else {
		headerEl = containerEl.Children().First()
		if headerEl.Find(option.selectorTorrent).Length() > 0 {
			// it's not header
			el := containerEl
			for el.Parent().Length() > 0 && el.Prev().Length() == 0 {
				el = el.Parent()
			}
			headerEl = el.Prev()
		}
		if headerEl.Length() == 0 {
			err = fmt.Errorf("cann't find headerEl")
			return
		}
		log.Tracef("nptr: header node=%v, id=%v, class=%v\n",
			headerEl.Get(0),
			headerEl.AttrOr("id", ""),
			headerEl.AttrOr("class", ""),
		)
	}
	for headerEl != nil && headerEl.Children().Length() == 1 {
		headerEl = headerEl.Children()
	}
	if headerEl != nil {
		headerEl.Children().Each(func(i int, s *goquery.Selection) {
			text := utils.DomSanitizedText(s)
			if text == "進度" || text == "进度" {
				fieldColumIndex["process"] = i
				return
			} else if text == "標題" || text == "标题" {
				fieldColumIndex["title"] = i
				return
			} else if text == "大小" {
				fieldColumIndex["size"] = i
				return
			} else if text == "时间" || text == "存活" {
				fieldColumIndex["time"] = i
				return
			} else if text == "上传" || text == "种子" {
				fieldColumIndex["seeders"] = i
				return
			} else if text == "下载" {
				fieldColumIndex["leechers"] = i
				return
			} else if text == "完成" {
				fieldColumIndex["snatched"] = i
				return
			}
			for field := range fieldColumIndex {
				if s.Find(`img[alt="`+field+`"],`+
					`img[alt="`+strings.ToUpper(field)+`"],`+
					`img[alt="`+utils.Capitalize(field)+`"]`).Length() > 0 {
					fieldColumIndex[field] = i
					break
				}
				if sortFields[field] != "" && s.Find(`a[href*="?sort=`+sortFields[field]+`&"],a[href*="&sort=`+sortFields[field]+`&"]`).Length() == 1 {
					fieldColumIndex[field] = i
					break
				}
			}
		})
	}
	var torrentBlocks *goquery.Selection
	if option.selectorTorrentBlock != "" {
		torrentBlocks = containerEl.Find(option.selectorTorrentBlock)
	} else {
		torrentBlocks = containerEl.Children()
	}
	torrentBlocks.Each(func(i int, s *goquery.Selection) {
		if s.Find(option.selectorTorrent).Length() == 0 {
			return
		}
		name := ""
		id := ""
		downloadUrl := ""
		size := int64(0)
		seeders := int64(0)
		leechers := int64(0)
		snatched := int64(0)
		time := int64(0)
		hnr := false
		downloadMultiplier := 1.0
		uploadMultiplier := 1.0
		discountEndTime := int64(-1)
		isActive := false
		processValueRegexp := regexp.MustCompile(`\d+(\.\d+)?%`)
		idRegexp := regexp.MustCompile(`[?&]id=(?P<id>\d+)`)
		text := utils.DomSanitizedText(s)

		s.Children().Each(func(i int, s *goquery.Selection) {
			for field, index := range fieldColumIndex {
				if index != i {
					continue
				}
				text := utils.DomSanitizedText(s)
				if field == "process" {
					if m := processValueRegexp.MatchString(text); m {
						isActive = true
					}
					continue
				}
				if field == "title" {
					continue
				}
				switch field {
				case "size":
					size, _ = utils.RAMInBytes(text)
				case "seeders":
					seeders = utils.ParseInt(text)
				case "leechers":
					leechers = utils.ParseInt(text)
				case "snatched":
					snatched = utils.ParseInt(text)
				case "time":
					time = utils.DomTime(s, option.location)
				}
			}
		})
		// lemonhd: href="details_movie.php?id=12345"
		titleEl := s.Find(option.selectorTorrentDetailsLink)
		if titleEl.Length() > 0 {
			name = titleEl.Text()
			// CloudFlare email obfuscation sometimes confuses with 0day torrent names such as "***-DIY@Audies"
			name = strings.ReplaceAll(name, "[email protected]", "")
			m := idRegexp.FindStringSubmatch(titleEl.AttrOr("href", ""))
			if m != nil {
				id = m[idRegexp.SubexpIndex("id")]
			}
		}
		downloadEl := s.Find(option.selectorTorrentDownloadLink)
		if downloadEl.Length() > 0 {
			downloadUrl = option.siteurl + downloadEl.AttrOr("href", "")
			m := idRegexp.FindStringSubmatch(downloadUrl)
			if m != nil {
				id = m[idRegexp.SubexpIndex("id")]
			}
		} else if id != "" {
			downloadUrl = option.siteurl + "download.php?id=" + fmt.Sprint(id)
		}
		if fieldColumIndex["time"] == -1 {
			if option.selectorTorrentTime != "" {
				time = utils.ExtractTime(utils.DomSelectorText(s, option.selectorTorrentTime), option.location)
			} else {
				time = utils.ExtractTime(text, option.location)
			}
		}
		if fieldColumIndex["seeders"] == -1 {
			if option.SelectorTorrentSeeders != "" {
				seeders = utils.ParseInt(utils.DomSelectorText(s, option.SelectorTorrentSeeders))
			}
		}
		if fieldColumIndex["leechers"] == -1 {
			if option.SelectorTorrentLeechers != "" {
				leechers = utils.ParseInt(utils.DomSelectorText(s, option.SelectorTorrentLeechers))
			}
		}
		if fieldColumIndex["snatched"] == -1 {
			if option.SelectorTorrentSnatched != "" {
				snatched = utils.ParseInt(utils.DomSelectorText(s, option.SelectorTorrentSnatched))
			}
		}
		if fieldColumIndex["size"] == -1 {
			if option.SelectorTorrentSize != "" {
				size, _ = utils.RAMInBytes(utils.DomSelectorText(s, option.SelectorTorrentSize))
			}
		}
		if s.Find(`*[title="H&R"],*[alt="H&R"]`).Length() > 0 {
			hnr = true
		}
		if s.Find(`*[alt="2X Free"]`).Length() > 0 {
			downloadMultiplier = 0
			uploadMultiplier = 2
		} else if s.Find(`*[title="免费"],*[title="免費"],*[alt="Free"],*[alt="FREE"]`).Length() > 0 ||
			domCheckTextTagExisting(s, "free") {
			downloadMultiplier = 0
		} else if option.SelectorTorrentFree != "" {
			text := strings.ToLower(utils.DomSelectorText(s, option.SelectorTorrentFree))
			if strings.Contains(text, "免费") || strings.Contains(text, "免費") || strings.Contains(text, "free") {
				downloadMultiplier = 0
			}
		}
		if s.Find(`*[title^="seeding"],*[title^="leeching"],*[title^="downloading"],*[title^="uploading"],*[title^="inactivity"]`).Length() > 0 {
			isActive = true
		} else if option.SelectorTorrentProcessBar != "" && s.Find(option.SelectorTorrentProcessBar).Length() > 0 {
			isActive = true
		}
		re := regexp.MustCompile(`(?i)(?P<free>(^|\s)(免费|免費|FREE)\s*)?(剩余|剩餘|限时|限時)(时间|時間)?\s*(?P<time>[YMDHMSymdhms年月周天小时時分种鐘秒\d]+)`)
		m := re.FindStringSubmatch(utils.DomRemovedSpecialCharsText(s))
		if m != nil {
			if m[re.SubexpIndex("free")] != "" {
				downloadMultiplier = 0
			}
			discountEndTime, _ = utils.ParseFutureTime(m[re.SubexpIndex("time")])
		}
		if discountEndTime <= 0 && globalDiscountEndTime > 0 {
			discountEndTime = globalDiscountEndTime
		}
		if name != "" && downloadUrl != "" {
			if siteName != "" {
				id = siteName + "." + id
			}
			torrents = append(torrents, site.Torrent{
				Name:               name,
				Id:                 id,
				Size:               size,
				DownloadUrl:        downloadUrl,
				Leechers:           leechers,
				Seeders:            seeders,
				Snatched:           snatched,
				Time:               time,
				HasHnR:             hnr,
				DownloadMultiplier: downloadMultiplier,
				UploadMultiplier:   uploadMultiplier,
				DiscountEndTime:    discountEndTime,
				IsActive:           isActive,
			})
		}
	})

	return
}

func domCommonParent(node1 *goquery.Selection, node2 *goquery.Selection) *goquery.Selection {
	for node1.Length() > 0 && node2.Length() > 0 && node1.Get(0) != node2.Get(0) {
		node1 = node1.Parent()
		node2 = node2.Parent()
	}
	if node1.Length() > 0 && node2.Length() > 0 && node1.Get(0) == node2.Get(0) {
		return node1
	}
	return nil
}

func domCheckTextTagExisting(node *goquery.Selection, str string) (existing bool) {
	str = strings.ToLower(str)
	node.Find("*").EachWithBreak(func(i int, s *goquery.Selection) bool {
		if s.Children().Length() > 0 {
			return true
		}
		if strings.ToLower(utils.DomSanitizedText(s)) == str {
			existing = true
			return false
		}
		return true
	})
	return
}
