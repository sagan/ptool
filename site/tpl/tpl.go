package tpl

// 站点模板

import (
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/utils"
)

var (
	SITES = map[string](*config.SiteConfigStruct){
		"hhanclub": &config.SiteConfigStruct{ // 憨憨
			Type: "nexusphp",
			Url:  "https://hhanclub.top/",
		},
		"leaves": &config.SiteConfigStruct{ // 红叶
			Type:     "nexusphp",
			Url:      "https://leaves.red/",
			BrushUrl: "https://leaves.red/special.php",
		},
		"mteam": &config.SiteConfigStruct{ // 馒头
			Type:     "nexusphp",
			Url:      "https://kp.m-team.cc/",
			BrushUrl: "https://kp.m-team.cc/adult.php",
		},
		"sharkpt": &config.SiteConfigStruct{ // 鲨鱼
			Type: "nexusphp",
			Url:  "https://sharkpt.net/",
		},
		"soulvoice": &config.SiteConfigStruct{ // 铃音
			Type: "nexusphp",
			Url:  "https://pt.soulvoice.club/",
		},
		"wintersakura": &config.SiteConfigStruct{ // 冬樱
			Type: "nexusphp",
			Url:  "https://wintersakura.net/",
		},
	}
)

func init() {
	for name := range SITES {
		site.Register(&site.RegInfo{
			Name:    name,
			Creator: create,
		})
	}
}

func create(name string, siteConfig *config.SiteConfigStruct, globalConfig *config.ConfigStruct) (
	site.Site, error) {
	sc := *siteConfig
	utils.Assign(&sc, SITES[name])
	return site.CreateSiteInternal(name, &sc, globalConfig)
}
