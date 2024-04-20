package nexusphp

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
)

const (
	SELECTOR_TORRENT               = `a[href^="download.php?"],a[href^="download?"]`
	SELECTOR_DOWNLOAD_LINK         = `a[href^="download.php?"],a[href^="download?"]`
	SELECTOR_DETAILS_LINK          = `a[href^="details.php?"],a[href^="details_"]`
	SELECTOR_TORRENTS_LIST_DEFAULT = `table.torrents > tbody`
	// xiaomlove/nexusphp paid torrent feature.
	// see https://github.com/xiaomlove/nexusphp/blob/php8/app/Repositories/TorrentRepository.php .
	// function getPaidIcon.
	SELECTOR_TORRENT_PAID = `span[title="收费种子"],span[title="收費種子"],span[title="Paid torrent"]`
	// skip NP download notice. see https://github.com/xiaomlove/nexusphp/blob/php8/public/download.php
	LETDOWN_QUERYSTRING = "letdown=1"
)

type TorrentsParserOption struct {
	location                       *time.Location
	idRegexp                       *regexp.Regexp
	npletdown                      bool
	globalHr                       bool
	torrentDownloadUrl             string
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
	selectorTorrentHnR             string
	selectorTorrentNoTraffic       string
	selectorTorrentNeutral         string
	selectorTorrentPaid            string
	selectorTorrentDiscountEndTime string
}

func parseTorrents(doc *goquery.Document, option *TorrentsParserOption,
	doctime int64, sitename string) (torrents []site.Torrent, err error) {
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
	if option.selectorTorrentPaid == "" {
		option.selectorTorrentPaid = SELECTOR_TORRENT_PAID
	}

	globalFree := false
	maybeGlobalFree := false
	globalDiscountEndTime := int64(0)
	globalDiscountLabels := doc.Find("p,h1,h2,span,b,i,a").FilterFunction(func(i int, s *goquery.Selection) bool {
		txt := util.DomRemovedSpecialCharsTextPreservingTime(s)
		re := regexp.MustCompile(`(全站|全局)\s*(?P<free>\dX Free|Free|免费|优惠)\s*生效`)
		if matches := re.FindStringSubmatch(txt); matches != nil {
			discountText := strings.ToLower(matches[re.SubexpIndex("free")])
			if strings.Contains(discountText, "free") || strings.Contains(discountText, "免费") {
				maybeGlobalFree = true
			}
			return true
		}
		return false
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
		for i := 0; globalDiscountLabel.Parent().Length() > 0 && globalDiscountEl.Prev().Length() == 0 && i < 3; i++ {
			globalDiscountEl = globalDiscountEl.Parent()
		}
		txt := util.DomRemovedSpecialCharsTextPreservingTime(globalDiscountEl)
		var time1, time2, offset int64
		time1, offset = util.ExtractTime(txt, option.location)
		if offset > 0 {
			time2, _ = util.ExtractTime(txt[offset:], option.location)
		}
		if time1 > 0 && time2 > 0 && doctime >= time1 && doctime < time2 {
			log.Tracef("Found global discount timespan: %s ~ %s", util.FormatTime(time1), util.FormatTime(time2))
			globalDiscountEndTime = time2
			globalFree = maybeGlobalFree
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
		containerElNode = util.MapMaxElementKey(commonParentsCnt)
		if containerElNode != nil {
			containerEl = nodeSelectionMap[containerElNode]
		}
	} else if torrentEls.Length() == 1 { // only one torrent found (e.g.: search result page)
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
		"category": -1,
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
			text := util.DomSanitizedText(s)
			if text == "类型" || text == "類型" || text == "分類" || text == "分类" {
				fieldColumIndex["category"] = i
				return
			} else if text == "進度" || text == "进度" {
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
					`img[alt="`+util.Capitalize(field)+`"]`).Length() > 0 {
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
	log.Tracef("np parse fieldColumIndex: %v", fieldColumIndex)
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
		var tags []string
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
		neutral := false
		processValueRegexp := regexp.MustCompile(`\d+(\.\d+)?%`)
		text := util.DomSanitizedText(s)
		var titleEl *goquery.Selection

		s.Children().Each(func(i int, s *goquery.Selection) {
			for field, index := range fieldColumIndex {
				if index != i {
					continue
				}
				text := util.DomSanitizedText(s)
				if field == "process" {
					if m := processValueRegexp.MatchString(text); m {
						isActive = true
					}
					continue
				}
				switch field {
				case "category":
					s.Find(`img[alt]`).Each(func(i int, s *goquery.Selection) {
						tag := s.AttrOr("alt", "")
						if tag == "" {
							tag = s.AttrOr("title", "")
						}
						if tag != "" {
							tags = append(tags, tag)
						}
					})
				case "size":
					size, _ = util.RAMInBytes(text)
				case "seeders":
					seeders = util.ParseInt(text)
				case "leechers":
					leechers = util.ParseInt(text)
				case "snatched":
					snatched = parseCountString(text)
				case "time":
					time = util.DomTime(s, option.location)
				case "name":
					titleEl = s.Find(option.selectorTorrentDetailsLink)
				}
			}
		})
		if titleEl == nil {
			titleEl = s.Find(option.selectorTorrentDetailsLink)
		}
		// 尽量不使用 a img 这种题图类型的标题元素(e.g.: M-Team)
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
			id = parseTorrentIdFromUrl(titleEl.AttrOr("href", ""), option.idRegexp)
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
						text := util.DomSanitizedText(s)
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
			downloadUrl = downloadEl.AttrOr("href", "")
			// @todo : add support for relative URL
			if !util.IsUrl(downloadUrl) {
				downloadUrl = option.siteurl + strings.TrimPrefix(downloadUrl, "/")
			}
			if option.npletdown {
				downloadUrl = util.AppendUrlQueryString(downloadUrl, LETDOWN_QUERYSTRING)
			}
			if id == "" {
				id = parseTorrentIdFromUrl(downloadUrl, option.idRegexp)
			}
		} else if id != "" {
			downloadUrl = option.siteurl + generateTorrentDownloadUrl(id, option.torrentDownloadUrl, option.npletdown)
		}
		if fieldColumIndex["time"] == -1 || time == 0 {
			if option.selectorTorrentTime != "" {
				time, _ = util.ExtractTime(util.DomSelectorText(s, option.selectorTorrentTime), option.location)
			} else {
				time, _ = util.ExtractTime(text, option.location)
			}
		}
		zeroSeederLeechers := seeders == 0 && leechers == 0
		if (fieldColumIndex["seeders"] == -1 || zeroSeederLeechers) && option.selectorTorrentSeeders != "" {
			seeders = util.ParseInt(util.DomSelectorText(s, option.selectorTorrentSeeders))
		}
		if (fieldColumIndex["leechers"] == -1 || zeroSeederLeechers) && option.selectorTorrentLeechers != "" {
			leechers = util.ParseInt(util.DomSelectorText(s, option.selectorTorrentLeechers))
		}
		if fieldColumIndex["snatched"] == -1 && option.selectorTorrentSnatched != "" {
			snatched = util.ParseInt(util.DomSelectorText(s, option.selectorTorrentSnatched))
		}
		if (fieldColumIndex["size"] == -1 || size <= 0) && option.selectorTorrentSize != "" {
			size, _ = util.RAMInBytes(util.DomSelectorText(s, option.selectorTorrentSize))
		}
		if s.Find(`*[title="H&R"],*[alt="H&R"],*[title="Hit and Run"]`).Length() > 0 {
			hnr = true
		} else if option.selectorTorrentHnR != "" && s.Find(option.selectorTorrentHnR).Length() > 0 {
			hnr = true
		}
		if s.Find(`*[alt="2X Free"]`).Length() > 0 {
			downloadMultiplier = 0
			uploadMultiplier = 2
		} else if s.Find(`*[title="免费"],*[title="免費"],*[alt="Free"],*[alt="FREE"],*[alt="free"]`).Length() > 0 ||
			domCheckTextTagExisting(s, "free") {
			downloadMultiplier = 0
		} else if domCheckTextTagExisting(s, "2xfree") {
			downloadMultiplier = 0
			uploadMultiplier = 2
		} else if option.selectorTorrentFree != "" && s.Find(option.selectorTorrentFree).Length() > 0 {
			downloadMultiplier = 0
		}
		if option.selectorTorrentPaid != "" && s.Find(option.selectorTorrentPaid).Length() > 0 {
			paid = true
		}
		if option.selectorTorrentNeutral != "" && s.Find(option.selectorTorrentNeutral).Length() > 0 {
			downloadMultiplier = 0
			uploadMultiplier = 0
			neutral = true
		} else if option.selectorTorrentNoTraffic != "" && s.Find(option.selectorTorrentNoTraffic).Length() > 0 {
			downloadMultiplier = 0
			uploadMultiplier = 0
		}
		if s.Find(`*[title^="seeding"],*[title^="leeching"],*[title^="downloading"],*[title^="uploading"],*[title^="inactivity"]`).Length() > 0 {
			isActive = true
		} else if option.selectorTorrentProcessBar != "" && s.Find(option.selectorTorrentProcessBar).Length() > 0 {
			isActive = true
		}
		if option.selectorTorrentDiscountEndTime != "" {
			discountEndTime, _ = util.ParseFutureTime(util.DomRemovedSpecialCharsText(s.Find(option.selectorTorrentDiscountEndTime)))
		} else {
			re := regexp.MustCompile(`(?i)(?P<free>(^|\s)(免费|免費|FREE)\s*)?(剩余|剩餘|限时|限時)(时间|時間)?\s*(?P<time>\d[\sYMDHMSymdhms年月周天小时時分种鐘秒\d]+[YMDHMSymdhms年月周天小时時分种鐘秒])`)
			m := re.FindStringSubmatch(util.DomRemovedSpecialCharsText(s))
			if m != nil {
				if m[re.SubexpIndex("free")] != "" {
					downloadMultiplier = 0
				}
				discountEndTime, _ = util.ParseFutureTime(m[re.SubexpIndex("time")])
			}
		}
		if discountEndTime <= 0 && globalDiscountEndTime > 0 {
			discountEndTime = globalDiscountEndTime
		}
		if downloadMultiplier == 1 && globalFree {
			downloadMultiplier = 0
		}
		if name != "" && (downloadUrl != "" || id != "") {
			if id != "" && sitename != "" {
				id = sitename + "." + id
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
				Neutral:            neutral,
				Tags:               tags,
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
		if strings.ToLower(util.DomSanitizedText(s)) == str {
			existing = true
			return false
		}
		return true
	})
	return
}

func parseCountString(str string) int64 {
	return util.ParseInt(strings.TrimSuffix(str, "次"))
}

func parseTorrentIdFromUrl(torrentUrl string, idRegexp *regexp.Regexp) (id string) {
	if idRegexp != nil {
		if m := idRegexp.FindStringSubmatch(torrentUrl); m != nil {
			id = m[idRegexp.SubexpIndex("id")]
		}
	}
	if id == "" {
		if urlObj, err := url.Parse(torrentUrl); err == nil {
			id = urlObj.Query().Get("id")
		}
	}
	return
}

// return absolute or relative (without leading "/"") torrent download url
func generateTorrentDownloadUrl(id string, torrentDownloadUrl string, letdown bool) string {
	downloadUrl := ""
	if torrentDownloadUrl != "" {
		downloadUrl = strings.ReplaceAll(torrentDownloadUrl, "{id}", id)
		downloadUrl = strings.TrimPrefix(downloadUrl, "/")
	} else {
		downloadUrl = "download.php?https=1&&id=" + id
		if letdown {
			downloadUrl += "&" + LETDOWN_QUERYSTRING
		}
	}
	return downloadUrl
}
