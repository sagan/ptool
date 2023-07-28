package nexusphp

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

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
	location                       *time.Location
	globalHr                       bool
	siteurl                        string
	selectorTorrentsListHeader     string
	selectorTorrentsList           string
	selectorTorrentBlock           string
	selectorTorrent                string
	selectorTorrentDownloadLink    string
	selectorTorrentDetailsLink     string
	selectorTorrentTime            string
	selectorTorrentSeeders         string
	selectorTorrentLeechers        string
	selectorTorrentSnatched        string
	selectorTorrentSize            string
	selectorTorrentProcessBar      string
	selectorTorrentFree            string
	selectorTorrentPaid            string
	selectorTorrentDiscountEndTime string
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

	globalDiscountEndTime := int64(0)
	globalDiscountLabels := doc.Find("p,span,b,i,a").FilterFunction(func(i int, s *goquery.Selection) bool {
		txt := utils.DomRemovedSpecialCharsTextPreservingTime(s)
		re := regexp.MustCompile(`(全站|全局)\s*(\dX Free|Free|优惠)\s*生效`)
		return re.MatchString(txt)
	})
	if globalDiscountLabels.Length() > 0 {
		var globalDiscountLabel *goquery.Selection
		globalDiscountLabels.EachWithBreak(func(i int, s *goquery.Selection) bool {
			if globalDiscountLabel == nil {
				globalDiscountLabel = s
			} else if globalDiscountLabel.Contains(s.Get(0)) {
				globalDiscountLabel = s
			} else {
				return false
			}
			return true
		})
		globalDiscountEl := globalDiscountLabel.Parent()
		for i := 0; globalDiscountEl.Prev().Length() == 0 && i < 3; i++ {
			globalDiscountEl = globalDiscountEl.Parent()
		}
		txt := utils.DomRemovedSpecialCharsTextPreservingTime(globalDiscountEl)
		var time1, time2, offset int64
		time1, offset = utils.ExtractTime(txt, option.location)
		if offset > 0 {
			time2, _ = utils.ExtractTime(txt[offset:], option.location)
		}
		if time1 > 0 && time2 > 0 && doctime >= time1 && doctime < time2 {
			log.Tracef("Found global discount timespan: %s ~ %s", utils.FormatTime(time1), utils.FormatTime(time2))
			globalDiscountEndTime = time2
		}
	}

	var containerElNode *html.Node
	var containerEl *goquery.Selection
	torrentEls := doc.Find(option.selectorTorrent)
	if option.selectorTorrentsList != "" {
		containerEl = doc.Find(option.selectorTorrentsList)
		if containerEl.Length() > 0 {
			containerElNode = containerEl.Get(0)
		}
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
		"name":     -1,
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
			} else if text == "標題" || text == "标题" || text == "Name" {
				fieldColumIndex["name"] = i
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
		description := ""
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
		paid := false
		processValueRegexp := regexp.MustCompile(`\d+(\.\d+)?%`)
		idRegexp := regexp.MustCompile(`[?&]id=(?P<id>\d+)`)
		text := utils.DomSanitizedText(s)
		var titleEl *goquery.Selection

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
				case "name":
					titleEl = s.Find(option.selectorTorrentDetailsLink)
				}
			}
		})
		if titleEl == nil {
			titleEl = s.Find(option.selectorTorrentDetailsLink)
		}
		// 尽量不使用 a img 这种题图类型的标题元素(eg. M-Team)
		titleTextEl := titleEl.FilterFunction(func(i int, s *goquery.Selection) bool {
			parentNode := s.Parent().Get(0)
			if parentNode.DataAtom == atom.Img || parentNode.DataAtom == atom.Image {
				return false
			}
			if s.Children().Length() == 1 {
				childNode := s.Children().Get(0)
				if childNode.DataAtom == atom.Img || childNode.DataAtom == atom.Image {
					return false
				}
			}
			return true
		})
		if titleTextEl.Length() > 0 {
			titleEl = titleTextEl.First()
		} else {
			titleEl = titleEl.First()
		}
		if titleEl.Length() > 0 {
			name = titleEl.Text()
			if name == "" {
				name = titleEl.AttrOr("title", "")
			}
			// CloudFlare email obfuscation sometimes confuses with 0day torrent names such as "***-DIY@Audies"
			name = strings.ReplaceAll(name, "[email protected]", "")
			m := idRegexp.FindStringSubmatch(titleEl.AttrOr("href", ""))
			if m != nil {
				id = m[idRegexp.SubexpIndex("id")]
			}
			// try to find np torrent subtitle that is after title and <br />
			foundBr := false
			foundSelf := false
			titleNode := titleEl.Get(0)
			titleEl.Parent().Contents().Each(func(i int, s *goquery.Selection) {
				if s.Get(0) == titleNode {
					foundSelf = true
				} else if foundSelf {
					if s.Get(0).DataAtom == atom.Br {
						foundBr = true
					} else if foundBr {
						text := utils.DomSanitizedText(s)
						if description != "" && text != "" {
							description += " "
						}
						description += text
					}
				}
			})
			// can NOT use the below way because Next() ignores text Nodes
			// for next := titleEl.Next(); next.Length() > 0; next = next.Next() {
			// }
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
				time, _ = utils.ExtractTime(utils.DomSelectorText(s, option.selectorTorrentTime), option.location)
			} else {
				time, _ = utils.ExtractTime(text, option.location)
			}
		}
		if fieldColumIndex["seeders"] == -1 {
			if option.selectorTorrentSeeders != "" {
				seeders = utils.ParseInt(utils.DomSelectorText(s, option.selectorTorrentSeeders))
			}
		}
		if fieldColumIndex["leechers"] == -1 {
			if option.selectorTorrentLeechers != "" {
				leechers = utils.ParseInt(utils.DomSelectorText(s, option.selectorTorrentLeechers))
			}
		}
		if fieldColumIndex["snatched"] == -1 {
			if option.selectorTorrentSnatched != "" {
				snatched = utils.ParseInt(utils.DomSelectorText(s, option.selectorTorrentSnatched))
			}
		}
		if fieldColumIndex["size"] == -1 {
			if option.selectorTorrentSize != "" {
				size, _ = utils.RAMInBytes(utils.DomSelectorText(s, option.selectorTorrentSize))
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
		} else if option.selectorTorrentFree != "" {
			text := strings.ToLower(utils.DomSelectorText(s, option.selectorTorrentFree))
			if strings.Contains(text, "免费") || strings.Contains(text, "免費") || strings.Contains(text, "free") {
				downloadMultiplier = 0
			}
		}
		if option.selectorTorrentPaid != "" && s.Find(option.selectorTorrentPaid).Length() > 0 {
			paid = true
		}
		if s.Find(`*[title^="seeding"],*[title^="leeching"],*[title^="downloading"],*[title^="uploading"],*[title^="inactivity"]`).Length() > 0 {
			isActive = true
		} else if option.selectorTorrentProcessBar != "" && s.Find(option.selectorTorrentProcessBar).Length() > 0 {
			isActive = true
		}
		if option.selectorTorrentDiscountEndTime != "" {
			discountEndTime, _ = utils.ParseFutureTime(utils.DomRemovedSpecialCharsText(s.Find(option.selectorTorrentDiscountEndTime)))
		} else {
			re := regexp.MustCompile(`(?i)(?P<free>(^|\s)(免费|免費|FREE)\s*)?(剩余|剩餘|限时|限時)(时间|時間)?\s*(?P<time>[YMDHMSymdhms年月周天小时時分种鐘秒\d]+)`)
			m := re.FindStringSubmatch(utils.DomRemovedSpecialCharsText(s))
			if m != nil {
				if m[re.SubexpIndex("free")] != "" {
					downloadMultiplier = 0
				}
				discountEndTime, _ = utils.ParseFutureTime(m[re.SubexpIndex("time")])
			}
		}
		if discountEndTime <= 0 && globalDiscountEndTime > 0 {
			discountEndTime = globalDiscountEndTime
		}
		if name != "" && (downloadUrl != "" || id != "") {
			if id != "" && siteName != "" {
				id = siteName + "." + id
			}
			torrents = append(torrents, site.Torrent{
				Name:               name,
				Description:        description,
				Id:                 id,
				Size:               size,
				DownloadUrl:        downloadUrl,
				Leechers:           leechers,
				Seeders:            seeders,
				Snatched:           snatched,
				Time:               time,
				HasHnR:             hnr || option.globalHr,
				DownloadMultiplier: downloadMultiplier,
				UploadMultiplier:   uploadMultiplier,
				DiscountEndTime:    discountEndTime,
				IsActive:           isActive,
				Paid:               paid,
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
