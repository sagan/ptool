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

type Site interface {
	GetSiteConfig() *config.SiteConfigStruct
	DownloadTorrent(url string) ([]byte, error)
	GetLatestTorrents(url string) ([]SiteTorrent, error)
}

type RegInfo struct {
	Name    string
	Creator func(*config.SiteConfigStruct, *config.ConfigStruct) (Site, error)
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

func CreateSite(name string) (Site, error) {
	siteConfig := config.GetSiteConfig(name)
	if siteConfig == nil {
		return nil, fmt.Errorf("site %s not existed", name)
	}
	regInfo, err := Find(siteConfig.Type)
	if err != nil {
		return nil, fmt.Errorf("unsupported site type %s", siteConfig.Type)
	}
	return regInfo.Creator(siteConfig, config.Get())
}

func Print(siteTorrents []SiteTorrent) {
	for _, siteTorrent := range siteTorrents {
		fmt.Printf(
			"%s, %s, %s, %d, %d, HR=%t\n",
			siteTorrent.Name,
			utils.FormatTime(siteTorrent.Time),
			utils.HumanSize(float64(siteTorrent.Size)),
			siteTorrent.Seeders,
			siteTorrent.Leechers,
			siteTorrent.HasHnR,
		)
	}
}

func init() {
}
