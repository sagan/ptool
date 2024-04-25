package get

import (
	"fmt"
	"net/url"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/cmd/cookiecloud"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
)

var (
	checkSite = false
	showAll   = false
	format    = ""
	profile   = ""
)

var command = &cobra.Command{
	Use:         "get {site | group | domain | url}...",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "cookiecloud.get"},
	Short:       "Get cookie for sites or domains from data of cookiecloud servers.",
	Long: `Get cookie for sites or domains from data of cookiecloud servers.
Each arg can be a site or group name, a domain (or IP), or a url. It will query cookiecloud servers
and display all found cookies of the corresponding arg in list.

If --all flag is NOT set (the default case), only cookies which path attribute constitutes the prefix of
the url's path will be included (in the case of arg being a domain or IP, it's path is assumed to be "/").
If --all flag is set, all cookies associated with the domain of arg will be included, the result is only
useful / meanful in the case of "js" format.

By default it will show site cookies in http request "Cookie" header format.
If --format flag is set to "js", it will show cookies in a JavaScript "document.cookie=''" code snippet format,
which is suitable for pasting to the browser developer console to set the cookies of corresponding site.`,
	Args: cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE: get,
}

func init() {
	cmd.AddEnumFlagP(command, &format, "format", "", &cmd.EnumFlag{
		Description: "Cookies output format",
		Options: [][2]string{
			{"http", `http request "Cookie" header`},
			{"js", `JavaScript 'document.cookie=""' code snippet`},
		},
	})
	command.Flags().BoolVarP(&showAll, "all", "a", false,
		"Show all cookies associated with the domain (no path checking)")
	command.Flags().BoolVarP(&checkSite, "check", "", false,
		`If arg is a sitename, check the validity of it's cookie. `+
			`Valid and invalid cookie will display a "✓" or "✕" flag respectively. `+
			`Note it will stop when the cookie of a site from any CookieCloud profile successed, `+
			`skip checking remaining cookies of that site that are fetched from other CookieCloud profiles`)
	command.Flags().StringVarP(&profile, "profile", "", "",
		"Comma-separated list. Set the used cookiecloud profile name(s). "+
			"If not set, All cookiecloud profiles in config will be used")
	cookiecloud.Command.AddCommand(command)
}

func get(cmd *cobra.Command, args []string) error {
	errorCnt := int64(0)
	cookiecloudProfiles := cookiecloud.ParseProfile(profile)
	if len(cookiecloudProfiles) == 0 {
		return fmt.Errorf("no cookiecloud profile specified or found")
	}
	cookiecloudDatas := []cookiecloud.Ccdata_struct{}
	for _, profile := range cookiecloudProfiles {
		data, err := cookiecloud.GetCookiecloudData(profile.Server, profile.Uuid, profile.Password,
			config.GetProxy(profile.Proxy), util.FirstNonZeroIntegerArg(config.Timeout, profile.Timeout))
		if err != nil {
			log.Errorf("Cookiecloud server %s (uuid %s) connection failed: %v\n", profile.Server, profile.Uuid, err)
			errorCnt++
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
	siteOrDomainOrUrls := config.ParseGroupAndOtherNames(args...)

	successSites := map[string]bool{}
	fmt.Printf("%-20s  %-20s  %-5s  %s\n", "Site/Url/Hostname", "CookieCloud", "Flags", "Cookie")
	for _, siteOrDomainOrUrl := range siteOrDomainOrUrls {
		domainOrUrl := ""
		var siteConfig *config.SiteConfigStruct
		if siteConfig = config.GetSiteConfig(siteOrDomainOrUrl); siteConfig != nil {
			siteInstance, err := site.CreateSiteInternal(siteOrDomainOrUrl, siteConfig, config.Get())
			if err != nil {
				log.Debugf("Failed to create site %s", siteOrDomainOrUrl)
			} else {
				domainOrUrl = util.ParseUrlHostname(siteInstance.GetSiteConfig().Url)
			}
		} else {
			domainOrUrl = siteOrDomainOrUrl
		}
		var cookieScope = domainOrUrl // the valid scope of cookies
		if domainOrUrl == "" {
			fmt.Printf("%-20s  %-20s  %-5s  %s\n",
				util.First(util.StringPrefixInWidth(siteOrDomainOrUrl, 20)), "", "", "// Error: empty hostname")
			errorCnt++
			continue
		} else if !util.IsUrl(domainOrUrl) && !util.IsHostname(domainOrUrl) {
			fmt.Printf("%-20s  %-20s  %-5s  %s\n",
				util.First(util.StringPrefixInWidth(siteOrDomainOrUrl, 20)), "", "", "// Error: invalid site, url or hostname")
			errorCnt++
			continue
		} else if util.IsUrl(domainOrUrl) {
			urlObj, err := url.Parse(domainOrUrl)
			if err == nil {
				cookieScope = urlObj.Hostname()
				if !showAll && urlObj.Path != "/" {
					cookieScope += urlObj.Path
				}
			}
		}
		if showAll {
			cookieScope += " <all>"
		}
		for _, cookiecloudData := range cookiecloudDatas {
			cookie, _, _ := cookiecloudData.Data.GetEffectiveCookie(domainOrUrl, showAll, format)
			if cookie == "" {
				log.Debugf("No cookie found for %s in cookiecloud %s", siteOrDomainOrUrl, cookiecloudData.Label)
				continue
			}
			checkFlag := 0
			if checkSite && siteConfig != nil && !successSites[siteOrDomainOrUrl] && !siteConfig.NoCookie {
				siteConfig.Cookie = cookie
				checkFlag = 1
				if siteInstance, err := site.CreateSiteInternal(siteOrDomainOrUrl, siteConfig, config.Get()); err != nil {
					log.Debugf("Failed to create site %s with cookie from %s", siteOrDomainOrUrl, cookiecloudData.Label)
				} else if sitestatus, err := siteInstance.GetStatus(); err != nil {
					log.Debugf("Failed to get site %s status with cookie from %s", siteOrDomainOrUrl, cookiecloudData.Label)
				} else if !sitestatus.IsOk() {
					log.Debugf("Cookie of site %s from cookiecloud %s is invalid", siteOrDomainOrUrl, cookiecloudData.Label)
				} else {
					successSites[siteOrDomainOrUrl] = true
					checkFlag = 2
				}
			}
			cookieStr := cookie + " // " + cookieScope
			var flags []string
			if checkSite && siteConfig != nil {
				if checkFlag == 0 {
					flags = append(flags, "-")
				} else if checkFlag == 1 {
					flags = append(flags, "✕")
				} else if checkFlag == 2 {
					flags = append(flags, "✓")
				}
			}
			fmt.Printf("%-20s  %-20s  %-5s  %s\n", util.First(util.StringPrefixInWidth(siteOrDomainOrUrl, 20)),
				util.First(util.StringPrefixInWidth(cookiecloudData.Label, 20)), strings.Join(flags, ""), cookieStr)
		}
	}

	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
