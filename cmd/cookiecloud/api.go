package cookiecloud

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/crypto"
)

type Ccdata_struct struct {
	Label string
	Sites []string
	Data  *CookiecloudData
}

type Cookie struct {
	Domain string
	Name   string
	Value  string
	Path   string
}

type CookieCloudBody struct {
	Uuid      string `json:"uuid,omitempty"`
	Encrypted string `json:"encrypted,omitempty"`
}

type CookiecloudData struct {
	// host => [{name,value,domain}...]
	Cookie_data map[string][]map[string]any `json:"cookie_data"`
}

// If proxy is empty, will try to get proxy from HTTP_PROXY & HTTPS_PROXY envs.
func GetCookiecloudData(server string, uuid string, password string,
	proxy string, timeout int64) (*CookiecloudData, error) {
	if server == "" || uuid == "" || password == "" {
		return nil, fmt.Errorf("all params of server,uuid,password must be provided")
	}
	if !strings.HasSuffix(server, "/") {
		server += "/"
	}
	if proxy == "" || proxy == "env" {
		proxy = util.ParseProxyFromEnv(server)
	}
	if timeout == 0 {
		timeout = config.DEFAULT_COOKIECLOUD_TIMEOUT
	}
	httpClient := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}
	if proxy != "" {
		proxyUrl, err := url.Parse(proxy)
		if err != nil {
			return nil, fmt.Errorf("failed to parse proxy %s: %v", proxy, err)
		}
		httpClient.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyUrl),
		}
	}
	var data *CookieCloudBody
	err := util.FetchJson(server+"get/"+uuid, &data, httpClient, nil)
	if err != nil || data == nil {
		return nil, fmt.Errorf("failed to get cookiecloud data: err=%v, null data=%t", err, data == nil)
	}
	keyPassword := crypto.Md5String(uuid, "-", password)[:16]
	decrypted, err := crypto.DecryptCryptoJsAesMsg(keyPassword, data.Encrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt data: err=%v", err)
	}
	var cookiecloudData *CookiecloudData
	err = json.Unmarshal(decrypted, &cookiecloudData)
	if err != nil || cookiecloudData == nil {
		return nil, fmt.Errorf("failed to parse decrypted data as json: err=%v", err)
	}
	return cookiecloudData, nil
}

// If all is false, only return cookies which is valid for both the hostname and path part of the urlOrDomain,
// in the case of urlOrDomain being a domain, it's path is assumed to be "/".
// If all is true, path check is skipped and all cookies which domain match will be included.
// format: "http" - http request "Cookie" header; "js" - JavaScript document.cookie="" code snippet
func (cookiecloudData *CookiecloudData) GetEffectiveCookie(urlOrDomain string, all bool, format string) (
	string, []*Cookie, error) {
	hostname := urlOrDomain
	path := "/"
	if util.IsUrl(urlOrDomain) {
		urlObj, err := url.Parse(urlOrDomain)
		if err != nil {
			return "", nil, fmt.Errorf("arg is not a valid url: %v", err)
		}
		hostname = urlObj.Hostname()
		path = urlObj.Path
	}
	if hostname == "" {
		return "", nil, fmt.Errorf("hostname can not be empty")
	}
	effectiveCookies := []*Cookie{}
	keys := []string{hostname, "." + hostname}
	for _, key := range keys {
		cookies, ok := cookiecloudData.Cookie_data[key]
		if !ok {
			continue
		}
		for _, cookie := range cookies {
			if cookie == nil {
				continue
			}
			cookieDomain, _ := cookie["domain"].(string)
			if cookieDomain != hostname && cookieDomain != "."+hostname {
				continue
			}
			cookiePath, _ := cookie["path"].(string)
			if cookiePath == "" {
				cookiePath = "/"
			}
			if !all && !strings.HasPrefix(path, cookiePath) {
				continue
			}
			// cookiecloud 导出的 cookies 里的 expirationDate 为 float 类型。意义不明确，暂不使用。
			cookieName, _ := cookie["name"].(string)
			cookieValue, _ := cookie["value"].(string)
			// RFC 似乎允许 empty cookie ?
			if cookieName == "" || cookieValue == "" {
				continue
			}
			effectiveCookies = append(effectiveCookies, &Cookie{
				Domain: cookieDomain,
				Path:   cookiePath,
				Name:   cookieName,
				Value:  cookieValue,
			})
		}
	}
	if len(effectiveCookies) == 0 {
		return "", nil, nil
	}
	if !all {
		sort.SliceStable(effectiveCookies, func(i, j int) bool {
			a := effectiveCookies[i]
			b := effectiveCookies[j]
			if a.Domain != b.Domain {
				return false
			}
			// longest path first
			if len(a.Path) != len(b.Path) {
				return len(a.Path) > len(b.Path)
			}
			return false
		})
		effectiveCookies = util.UniqueSliceFn(effectiveCookies, func(cookie *Cookie) string {
			return cookie.Name
		})
	}
	cookieStr := ""
	if format == "http" {
		sep := ""
		for _, cookie := range effectiveCookies {
			cookieStr += sep + cookie.Name + "=" + cookie.Value
			sep = "; "
		}
	} else if format == "js" {
		for _, cookie := range effectiveCookies {
			// max-age (seconds): 100 years. While Chrome will cap it to max 400 days
			cookieStr += `document.cookie='` + cookie.Name + "=" + cookie.Value +
				"; path=" + cookie.Path + `; max-age=3153600000` + `';`
		}
	} else {
		return "", nil, fmt.Errorf("invalid format %s", format)
	}
	return cookieStr, effectiveCookies, nil
}

var cdnCookies = []string{"cf_clearance"}

// Check whether this cookie is set by the CDN or similar reverse-proxy services,
// which is not associated with authentication & authorization.
func (cookie *Cookie) IsCDN() bool {
	return slices.Index(cdnCookies, cookie.Name) != -1
}
