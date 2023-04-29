package site

import (
	"fmt"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/utils"
)

type Torrent struct {
	Name               string
	Id                 string // optional torrent id in the site
	InfoHash           string
	DownloadUrl        string
	DownloadMultiplier float64
	UploadMultiplier   float64
	DiscountEndTime    int64
	Time               int64 // torrent timestamp
	Size               int64
	IsSizeAccurate     bool
	Seeders            int64
	Leechers           int64
	Snatched           int64
	HasHnR             bool // true if has any type of HR
	IsActive           bool // true if torrent is as already downloading / seeding
}

type Status struct {
	UserName            string
	UserDownloaded      int64
	UserUploaded        int64
	TorrentsSeedingCnt  int64
	TorrentsLeechingCnt int64
}

type Site interface {
	GetName() string
	GetSiteConfig() *config.SiteConfigStruct
	DownloadTorrent(url string) (content []byte, filename string, err error)
	DownloadTorrentById(id string) (content []byte, filename string, err error)
	GetLatestTorrents(full bool) ([]Torrent, error)
	GetAllTorrents(sort string, desc bool, pageMarker string) (torrents []Torrent, nextPageMarker string, err error)
	SearchTorrents(keyword string) ([]Torrent, error)
	GetStatus() (*Status, error)
	PurgeCache()
}

type RegInfo struct {
	Name    string
	Aliases []string
	Creator func(string, *config.SiteConfigStruct, *config.ConfigStruct) (Site, error)
}

type SiteCreator func(*RegInfo) (Site, error)

var (
	registryMap = make(map[string](*RegInfo))
)

func Register(regInfo *RegInfo) {
	registryMap[regInfo.Name] = regInfo
	for _, alias := range regInfo.Aliases {
		registryMap[alias] = regInfo
	}
}

func CreateSiteInternal(name string,
	siteConfig *config.SiteConfigStruct, config *config.ConfigStruct) (Site, error) {
	regInfo := registryMap[siteConfig.Type]
	if regInfo == nil {
		return nil, fmt.Errorf("unsupported site type %s", name)
	}
	return regInfo.Creator(name, siteConfig, config)
}

func GetConfigSiteReginfo(name string) *RegInfo {
	for _, siteConfig := range config.Get().Sites {
		if siteConfig.GetName() == name {
			return registryMap[siteConfig.Type]
		}
	}
	return nil
}

func CreateSite(name string) (Site, error) {
	for _, siteConfig := range config.Get().Sites {
		if siteConfig.GetName() == name {
			return CreateSiteInternal(name, &siteConfig, config.Get())
		}
	}
	return nil, fmt.Errorf("site %s not found", name)
}

func Print(siteTorrents []Torrent) {
	for _, siteTorrent := range siteTorrents {
		fmt.Printf(
			"%s, %s, %s, %d, %d, HR=%t\n",
			siteTorrent.Name,
			utils.FormatTime(siteTorrent.Time),
			utils.BytesSize(float64(siteTorrent.Size)),
			siteTorrent.Seeders,
			siteTorrent.Leechers,
			siteTorrent.HasHnR,
		)
	}
}

func PrintTorrents(torrents []Torrent, filter string, now int64, noHeader bool) {
	if !noHeader {
		fmt.Printf("%-40s  %10s  %-13s  %19s  %4s  %4s  %4s  %10s  %2s\n", "Name", "Size", "Free", "Time", "↑S", "↓L", "✓C", "ID", "P")
	}
	for _, torrent := range torrents {
		if filter != "" && !utils.ContainsI(torrent.Name, filter) {
			continue
		}
		freeStr := ""
		if torrent.HasHnR {
			freeStr += "!"
		}
		if torrent.DownloadMultiplier == 0 {
			freeStr += "✓"
		} else {
			freeStr += "✕"
		}
		if torrent.DiscountEndTime > 0 {
			freeStr += fmt.Sprintf("(%s)", utils.FormatDuration(torrent.DiscountEndTime-now))
		}
		if torrent.UploadMultiplier > 1 {
			freeStr = fmt.Sprintf("%1.1f", torrent.UploadMultiplier) + freeStr
		}
		name := torrent.Name
		process := "-"
		if torrent.IsActive {
			process = "0%"
		}
		utils.PrintStringInWidth(name, 40, true)
		fmt.Printf("  %10s  %-13s  %19s  %4s  %4s  %4s  %10s  %2s\n",
			utils.BytesSize(float64(torrent.Size)),
			freeStr,
			utils.FormatTime(torrent.Time),
			fmt.Sprint(torrent.Seeders),
			fmt.Sprint(torrent.Leechers),
			fmt.Sprint(torrent.Snatched),
			torrent.Id,
			process,
		)
	}
}

func init() {
}
