package importsites

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/sagan/ptool/cmd/cookiecloud"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/site/tpl"
	"github.com/sagan/ptool/util"
)

var (
	doAction = false
	profile  = ""
)

var command = &cobra.Command{
	Use:         "import",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "cookiecloud.import"},
	Short:       "Import sites from cookies of cookiecloud servers.",
	Long: `Import sites from cookies of cookiecloud servers.
It will get latest cookies from cookiecloud servers, find sites that do NOT exist in config file currently,
Test their cookies are valid, then add them to config file.

It will ask for confirm before updating config file, unless --do flag is set.
Be aware that all existing comments in config file will be LOST when updating config file.`,
	RunE: importsites,
}

func init() {
	command.Flags().BoolVarP(&doAction, "do", "", false, "Do update the config file without confirm. Be aware that all existing comments in config file will be LOST")
	command.Flags().StringVarP(&profile, "profile", "", "", "Comma-separated string, Set the used cookiecloud profile name(s). If not set, All cookiecloud profiles in config will be used")
	cookiecloud.Command.AddCommand(command)
}

func importsites(cmd *cobra.Command, args []string) error {
	cntError := int64(0)
	cookiecloudProfiles := cookiecloud.ParseProfile(profile)
	if len(cookiecloudProfiles) == 0 {
		return fmt.Errorf("no cookiecloud profile specified or found")
	}
	cookiecloudDatas := []cookiecloud.Ccdata_struct{}
	for _, profile := range cookiecloudProfiles {
		data, err := cookiecloud.GetCookiecloudData(profile.Server, profile.Uuid, profile.Password, profile.Proxy)
		if err != nil {
			log.Errorf("Cookiecloud server %s (uuid %s) connection failed: %v\n", profile.Server, profile.Uuid, err)
			cntError++
		} else {
			log.Infof("Cookiecloud server %s (uuid %s) connection ok: cookies of %d domains found\n",
				profile.Server, profile.Uuid, len(data.Cookie_data))
			cookiecloudDatas = append(cookiecloudDatas, cookiecloud.Ccdata_struct{
				Label: fmt.Sprintf("%s-%s", util.GetUrlDomain(profile.Server), profile.Uuid),
				Sites: profile.Sites,
				Data:  data,
			})
		}
	}
	if len(cookiecloudDatas) == 0 {
		return fmt.Errorf("no cookiecloud server can be connected")
	}

	addSites := []*config.SiteConfigStruct{}
	tplExistingFlags := map[string]bool{}
	for _, tplname := range tpl.SITENAMES {
		tplInfo := tpl.SITES[tplname]
		for _, site := range config.Get().Sites {
			if site.Type == tplname || slices.Index(tplInfo.Aliases, site.Type) != -1 {
				tplExistingFlags[tplname] = true
				break
			}
		}
		if sitename, _ := tpl.GuessSiteByDomain(util.ParseUrlHostname(tplInfo.Url), ""); sitename != "" {
			tplExistingFlags[tplname] = true
		}
	}
	for _, cookiecloudData := range cookiecloudDatas {
		for _, tplname := range tpl.SITENAMES {
			if tplExistingFlags[tplname] {
				continue
			}
			cookie, _ := cookiecloudData.Data.GetEffectiveCookie(tpl.SITES[tplname].Url, false, "http")
			if cookie == "" {
				continue
			}
			newsiteconfig := &config.SiteConfigStruct{Type: tplname, Cookie: cookie}
			siteInstance, err := site.CreateSiteInternal(tplname, newsiteconfig, config.Get())
			if err != nil {
				log.Debugf("New Site %s from cookiecloud %s is invalid (create instance error: %v",
					tplname, cookiecloudData.Label, err)
				continue
			}
			sitestatus, err := siteInstance.GetStatus()
			if err != nil {
				log.Debugf("New Site %s from cookiecloud %s is invalid (get status error: %v",
					tplname, cookiecloudData.Label, err)
				continue
			}
			log.Infof("✓✓New site %s from cookiecloud %s is valid (username: %s)",
				tplname, cookiecloudData.Label, sitestatus.UserName)
			sitename := ""
			if config.GetSiteConfig(tplname) != nil {
				i := 1
				for {
					sitename = fmt.Sprint(tplname, i)
					if config.GetSiteConfig(sitename) == nil {
						break
					}
					i++
				}
			}
			log.Infof("Add new site type=%s, name=%s", tplname, sitename)
			addSites = append(addSites, &config.SiteConfigStruct{
				Name:   sitename,
				Type:   tplname,
				Cookie: cookie,
			})
			tplExistingFlags[tplname] = true
		}
	}

	if len(addSites) > 0 {
		fmt.Printf("✓new sites found (%d): %s", len(addSites),
			strings.Join(util.Map(addSites, func(site *config.SiteConfigStruct) string {
				return site.Type
			}), ", "))

		configFile := fmt.Sprintf("%s/%s", config.ConfigDir, config.ConfigFile)
		fmt.Printf("\n")
		if !doAction && !util.AskYesNoConfirm(
			fmt.Sprintf("Will update the config file (%s). Be aware that all existing comments will be LOST",
				configFile)) {
			return fmt.Errorf("abort")
		}
		config.UpdateSites(addSites)
		err := config.Set()
		if err == nil {
			fmt.Printf("Successfully update config file %s", configFile)
		} else {
			log.Fatalf("Failed to update config file %s : %v", configFile, err)
		}
	} else {
		fmt.Printf("!No new sites found in cookiecloud datas")
	}

	if cntError > 0 {
		return fmt.Errorf("%d errors", cntError)
	}
	return nil
}