package site

import (
	"fmt"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/utils"
)

type SiteTorrent struct {
	Name               string
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
	GetLatestTorrents(url string) ([]SiteTorrent, error)
	GetStatus() (*Status, error)
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

func Print(siteTorrents []SiteTorrent) {
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

func init() {
}
