package config

import (
	"log"
	"os"
	"sync"

	"github.com/sagan/ptool/utils"
	"gopkg.in/yaml.v3"
)

const (
	DEFAULT_CLIENT_BRUSH_MIN_DISK_SPACE         = int64(5 * 1024 * 1024 * 1024)
	DEFAULT_CLIENT_BRUSH_SLOW_UPLOAD_SPEED_TIER = int64(100 * 1024)
	DEFAULT_SITE_TORRENT_UPLOAD_SPEED_LIMIT     = int64(10 * 1024 * 1024)
)

type ClientConfigStruct struct {
	Name                          string `yaml:"name"`
	Type                          string `yaml:"type"`
	Url                           string `yaml:"url"`
	Username                      string `yaml:"username"`
	Password                      string `yaml:"password"`
	BrushMinDiskSpace             string `yaml:"brushMinDiskSpace"`
	BrushSlowUploadSpeedTier      string `yaml:"brushSlowUploadSpeedTier"`
	BrushMinDiskSpaceValue        int64
	BrushSlowUploadSpeedTierValue int64
}

type SiteConfigStruct struct {
	Name                         string `yaml:"name"`
	Type                         string `yaml:"type"`
	Url                          string `yaml:"url"`
	BrushUrl                     string `yaml:"brushUrl"`
	Cookie                       string `yaml:"cookie"`
	TorrentUploadSpeedLimit      string `yaml:"uploadSpeedLimit"`
	TorrentUploadSpeedLimitValue int64
}

type ConfigStruct struct {
	IyuuToken string               `yaml:"iyuutoken"`
	Clients   []ClientConfigStruct `yaml:"clients"`
	Sites     []SiteConfigStruct   `yaml:"sites"`
}

var (
	ConfigFile                 = ""
	ConfigLoaded               = false
	Config       *ConfigStruct = &ConfigStruct{}

	mu sync.Mutex
)

func init() {

}

func Get() *ConfigStruct {
	if !ConfigLoaded {
		mu.Lock()
		if !ConfigLoaded {
			log.Printf("Read config file %s", ConfigFile)
			file, err := os.ReadFile(ConfigFile)
			if err == nil {
				err = yaml.Unmarshal(file, &Config)
				if err != nil {
					log.Print(err)
				}
			}
			for i, client := range Config.Clients {
				v, err := utils.RAMInBytes(client.BrushMinDiskSpace)
				if err != nil || v <= 0 {
					v = DEFAULT_CLIENT_BRUSH_MIN_DISK_SPACE
				}
				Config.Clients[i].BrushMinDiskSpaceValue = v

				v, err = utils.RAMInBytes(client.BrushSlowUploadSpeedTier)
				if err != nil || v <= 0 {
					v = DEFAULT_CLIENT_BRUSH_SLOW_UPLOAD_SPEED_TIER
				}
				Config.Clients[i].BrushSlowUploadSpeedTierValue = v
			}
			for i, site := range Config.Sites {
				v, err := utils.RAMInBytes(site.TorrentUploadSpeedLimit)
				if err != nil || v <= 0 {
					v = DEFAULT_SITE_TORRENT_UPLOAD_SPEED_LIMIT
				}
				Config.Sites[i].TorrentUploadSpeedLimitValue = v
			}
			ConfigLoaded = true
		}
		mu.Unlock()
	}
	return Config
}

func GetClientConfig(name string) *ClientConfigStruct {
	for _, client := range Get().Clients {
		if client.Name == name {
			return &client
		}
	}
	return nil
}

func GetSiteConfig(name string) *SiteConfigStruct {
	for _, site := range Get().Sites {
		if site.Name == name {
			return &site
		}
	}
	return nil
}
