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
	DownloadTorrent(url string) ([]byte, error)
	DownloadTorrentById(id string) ([]byte, error)
	GetLatestTorrents(full bool) ([]Torrent, error)
	SearchTorrents(keyword string) ([]Torrent, error)
	GetStatus() (*Status, error)
	PurgeCache()
}

type RegInfo struct {
	Name    string
	Creator func(string, *config.SiteConfigStruct, *config.ConfigStruct) (Site, error)
}

type SiteCreator func(*RegInfo) (Site, error)

var (
	Registry []*RegInfo = make([]*RegInfo, 0)
)

func Register(regInfo *RegInfo) {
	Registry = append(Registry, regInfo)
}

func Find(name string) (*RegInfo, error) {
	for _, item := range Registry {
		if item.Name == name {
			return item, nil
		}
	}
	return nil, fmt.Errorf("didn't find site %q", name)
}

func CreateSiteInternal(name string,
	siteConfig *config.SiteConfigStruct, config *config.ConfigStruct) (Site, error) {
	regInfo, err := Find(siteConfig.Type)
	if err != nil {
		return nil, fmt.Errorf("unsupported site type %s", siteConfig.Type)
	}
	return regInfo.Creator(name, siteConfig, config)
}

func SiteExists(name string) bool {
	siteConfig := config.GetSiteConfig(name)
	return siteConfig != nil
}

func CreateSite(name string) (Site, error) {
	siteConfig := config.GetSiteConfig(name)
	if siteConfig == nil {
		return nil, fmt.Errorf("site %s not existed", name)
	}
	return CreateSiteInternal(name, siteConfig, config.Get())
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

func PrintTorrents(torrents []Torrent, filter string, now int64) {
	fmt.Printf("%-40s  %10s  %-13s  %19s  %4s  %4s  %4s  %10s  %2s\n", "Name", "Size", "Free", "Time", "↑S", "↓L", "✓C", "ID", "P")
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
		if len(name) > 37 {
			name = name[:37] + "..."
		}
		process := "-"
		if torrent.IsActive {
			process = "0%"
		}

		fmt.Printf("%-40s  %10s  %-13s  %19s  %4s  %4s  %4s  %10s  %2s\n",
			name,
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
