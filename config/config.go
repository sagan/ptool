package config

import (
	"log"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

type ClientConfigStruct struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Url      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type SiteConfigStruct struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Url      string `yaml:"url"`
	BrushUrl string `yaml:"brushUrl"`
	Cookie   string `yaml:"cookie"`
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
			log.Printf("read config file %s", ConfigFile)
			file, err := os.ReadFile(ConfigFile)
			if err == nil {
				err = yaml.Unmarshal(file, &Config)
				log.Print(err)
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
