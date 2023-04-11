package nexusphp

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	cloudflarebp "github.com/DaRealFreak/cloudflare-bp-go"
	"github.com/PuerkitoBio/goquery"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
	log "github.com/sirupsen/logrus"
)

type Site struct {
	Name               string
	Location           *time.Location
	SiteConfig         *config.SiteConfigStruct
	Config             *config.ConfigStruct
	HttpClient         *http.Client
	siteStatus         *site.Status
	latestSiteTorrents []site.Torrent
	datatime           int64
}

func (npclient *Site) PurgeCache() {
	npclient.datatime = 0
}

func (npclient *Site) GetName() string {
	return npclient.Name
}

func (npclient *Site) GetSiteConfig() *config.SiteConfigStruct {
	return npclient.SiteConfig
}

func (npclient *Site) DownloadTorrent(url string) ([]byte, error) {
	res, err := utils.FetchUrl(url, npclient.SiteConfig.Cookie, npclient.HttpClient)
	if err != nil {
		return nil, fmt.Errorf("can not fetch torrents from site: %v", err)
	}
	defer res.Body.Close()
	return io.ReadAll(res.Body)
}

func (npclient *Site) DownloadTorrentById(id string) ([]byte, error) {
	res, err := utils.FetchUrl(npclient.SiteConfig.Url+"download.php?https=1&id="+fmt.Sprint(id), npclient.SiteConfig.Cookie, npclient.HttpClient)
	if err != nil {
		return nil, fmt.Errorf("can not fetch torrents from site: %v", err)
	}
	defer res.Body.Close()
	return io.ReadAll(res.Body)
}

func (npclient *Site) GetStatus() (*site.Status, error) {
	err := npclient.sync()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch site data: %v", err)
	}
	return npclient.siteStatus, nil
}

func (npclient *Site) GetLatestTorrents() ([]site.Torrent, error) {
	err := npclient.sync()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch site data: %v", err)
	}
	return npclient.latestSiteTorrents, nil
}

func (npclient *Site) sync() error {
	if npclient.datatime > 0 {
		return nil
	}
	url := npclient.SiteConfig.BrushUrl
	if url == "" {
		url = npclient.SiteConfig.Url + "torrents.php"
	}
	res, err := utils.FetchUrl(url, npclient.SiteConfig.Cookie, npclient.HttpClient)
	if err != nil {
		return fmt.Errorf("can not fetch site data %v", err)
	}
	defer res.Body.Close()
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return fmt.Errorf("failed to parse site page DOM, error: %v", err)
	}
	npclient.datatime = utils.Now()

	siteStatus := &site.Status{}
	infoTr := doc.Find("#info_block, .m_nav").First()
	infoTxt := infoTr.Text()
	infoTxt = strings.ReplaceAll(infoTxt, "\n", " ")
	infoTxt = strings.ReplaceAll(infoTxt, "\r", " ")
	re := regexp.MustCompile(`(上傳量|上傳|上传量|上传)[：:\s]+(?P<s>[.\s0-9KMGTEPBkmgtepib]+)`)
	m := re.FindStringSubmatch(infoTxt)
	if m != nil {
		ss := strings.ReplaceAll(m[re.SubexpIndex("s")], " ", "")
		s, _ := utils.RAMInBytes(ss)
		siteStatus.UserUploaded = s
	}
	re = regexp.MustCompile(`(下載量|下載|下载量|下载)[：:\s]+(?P<s>[.\s0-9KMGTEPBkmgtepib]+)`)
	m = re.FindStringSubmatch(infoTxt)
	if m != nil {
		ss := strings.ReplaceAll(m[re.SubexpIndex("s")], " ", "")
		s, _ := utils.RAMInBytes(ss)
		siteStatus.UserDownloaded = s
	}
	userEl := doc.Find("*[href*=\"userdetails.php?\"]").First()
	if userEl.Length() > 0 {
		siteStatus.UserName = userEl.Text()
	}
	// possibly parsing error or some problem
	if siteStatus.UserName == "" && siteStatus.UserDownloaded == 0 && siteStatus.UserUploaded == 0 {
		if log.GetLevel() >= log.TraceLevel {
			log.Tracef("Site GetStatus got no data, html: %s", utils.DomHtml(doc.Find("html")))
		}
	}
	npclient.siteStatus = siteStatus

	torrents := []site.Torrent{}
	defer func() {
		if len(torrents) == 0 && log.GetLevel() >= log.TraceLevel {
			log.Tracef("LatestTorrents page html: %s", utils.DomHtml(doc.Find("html")))
		}
	}()
	headerTr := doc.Find("table.torrents > tbody > tr").First()
	if headerTr.Length() == 0 {
		return nil
	}
	fieldColumIndex := map[string]int{
		"time":     -1,
		"size":     -1,
		"seeders":  -1,
		"leechers": -1,
		"snatched": -1,
		"title":    -1,
		"process":  -1,
	}
	headerTr.Children().Each(func(i int, s *goquery.Selection) {
		text := utils.DomSanitizedText(s)
		if text == "進度" || text == "进度" {
			fieldColumIndex["process"] = i
			return
		}
		if text == "標題" || text == "标题" {
			fieldColumIndex["title"] = i
			return
		}
		for field := range fieldColumIndex {
			if s.Find(`img[alt="`+field+`"],`+`img[alt="`+strings.ToUpper(field)+`"],`+`img[alt="`+utils.Capitalize(field)+`"]`).Length() > 0 {
				fieldColumIndex[field] = i
				break
			}
		}
	})
	doc.Find("table.torrents > tbody > tr").Each(func(i int, s *goquery.Selection) {
		if i == 0 {
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
		var error error = nil
		processValueRegexp := regexp.MustCompile(`\d+(\.\d+)?%`)
		idRegexp := regexp.MustCompile(`[?&]id=(?P<id>\d+)`)

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
					title := s.Find("*[title]").AttrOr("title", "")
					time, error = utils.ParseTime(title, npclient.Location)
					if error == nil {
						break
					}
					time, error = utils.ParseTime(text, npclient.Location)
				}
			}
		})
		// lemonhd: href="details_movie.php?id=12345"
		titleEl := s.Find(`a[href^="details.php?"],a[href^="details_"]`)
		if titleEl.Length() > 0 {
			name = titleEl.Text()
			name = strings.ReplaceAll(name, "[email protected]", "") // CloudFlare email obfuscation sometimes confuses with 0day torrent names such as "***-DIY@Audies"
		}
		downloadEl := s.Find("a[href^=\"download.php?\"]")
		if downloadEl.Length() > 0 {
			downloadUrl = npclient.SiteConfig.Url + downloadEl.AttrOr("href", "")
			m := idRegexp.FindStringSubmatch(downloadUrl)
			if m != nil {
				id = m[idRegexp.SubexpIndex("id")]
			}
		}
		if s.Find(`*[title="H&R"],*[alt="H&R"]`).Length() > 0 {
			hnr = true
		}
		if s.Find(`*[title="免费"],*[title="免費"],*[alt="Free"],*[alt="FREE"],*[alt="2X Free"]`).Length() > 0 {
			downloadMultiplier = 0
		}
		if s.Find(`*[title^="seeding"],*[title^="leeching"],*[title^="downloading"],*[title^="uploading"]`).Length() > 0 {
			isActive = true
		}
		re := regexp.MustCompile(`(?P<free>(^|\s|\(|\[（【)(免费|免費)\s*)?(剩余|剩餘)(时间|時間)?[：:\s]*(?P<time>[YMDHMSymdhms年月天小时時分种鐘秒\d]+)`)
		m := re.FindStringSubmatch(utils.DomSanitizedText(s))
		if m != nil {
			if m[re.SubexpIndex("free")] != "" {
				downloadMultiplier = 0
			}
			discountEndTime, _ = utils.ParseFutureTime(m[re.SubexpIndex("time")])
		}
		if name != "" && downloadUrl != "" {
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
	npclient.latestSiteTorrents = torrents
	return nil
}

func NewSite(name string, siteConfig *config.SiteConfigStruct, config *config.ConfigStruct) (site.Site, error) {
	if siteConfig.Cookie == "" {
		return nil, fmt.Errorf("cann't create site: no cookie provided")
	}
	location, err := time.LoadLocation(siteConfig.Timezone)
	if err != nil {
		return nil, fmt.Errorf("Invalid site timezone: %s", siteConfig.Timezone)
	}
	httpClient := &http.Client{}
	httpClient.Transport = cloudflarebp.AddCloudFlareByPass(httpClient.Transport)
	client := &Site{
		Name:       name,
		Location:   location,
		SiteConfig: siteConfig,
		Config:     config,
		HttpClient: httpClient,
	}
	return client, nil
}

func init() {
	site.Register(&site.RegInfo{
		Name:    "nexusphp",
		Creator: NewSite,
	})
}

var (
	_ site.Site = (*Site)(nil)
)
