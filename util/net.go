package util

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/Noooste/azuretls-client"
	log "github.com/sirupsen/logrus"
)

type ImpersonateProfile struct {
	// "chrome", "firefox", "opera", "safari", "edge", "ios", "android"
	Navigator     string
	Ja3           string
	H2fingerpring string
	Headers       [][]string // use "\n" as placeholder for order; use "" (empty) to delete a header
	Comment       string
}

const (
	// 默认对站点的 http 请求模仿最新版 Chrome Windows 11 x64 en-US 环境
	DEFAULT_IMPERSONATE = "chrome120"
	// header 占位符。用于保证实际发送 headers 的顺序
	HTTP_HEADER_PLACEHOLDER = "\n"
)

// 对站点的 http 请求模仿真实浏览器环境。包括：
// TLS Ja3 指纹、http2 指纹、http headers。
// 查看当前 http 客户端的 ja3, http2 指纹, http headers 等信息:
// https://tls.peet.ws/api/all (该网站生成的ja3可能有问题),
// https://tools.scrapfly.io/api/fp/anything ,
// https://scrapfly.io/web-scraping-tools/ja3-fingerprint (建议用这个 ja3).
var ImpersonateProfiles = map[string]*ImpersonateProfile{
	"chrome120": {
		Navigator: "chrome",
		Comment:   "Chrome 120 on Windows 11 x64 en-US",
		// TLS ja3 指纹。参考: https://scrapfly.io/blog/how-to-avoid-web-scraping-blocking-tls/ .
		// Ja3 should be generated without the "TLS Session has been resurected" warning
		Ja3: "772,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,65281-45-11-65037-18-5-51-0-23-27-43-16-10-35-17513-13,29-23-24,0",
		// akamai_fingerprint 格式。http2 指纹参考: https://lwthiker.com/networks/2022/06/17/http2-fingerprinting.html .
		H2fingerpring: "1:65536,2:0,4:6291456,6:262144|15663105|0|m,a,s,p",
		// 请求默认 http headers。有序！
		Headers: [][]string{
			{"Cache-Control", "max-age=0"},
			{"Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`},
			{"Sec-Ch-Ua-Mobile", `?0`},
			{"Sec-Ch-Ua-Platform", `"Windows"`},
			{"Upgrade-Insecure-Requests", "1"},
			{"User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"},
			{"Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"},
			{"Sec-Fetch-Site", `none`},
			{"Sec-Fetch-Mode", `navigate`},
			{"Sec-Fetch-User", `?1`},
			{"Sec-Fetch-Dest", `document`},
			{"Accept-Encoding", "gzip, deflate, br"},
			{"Accept-Language", "en-US,en;q=0.9"},
			{"Cookie", HTTP_HEADER_PLACEHOLDER},
		},
	},
	"firefox121": {
		Navigator: "firefox",
		Comment:   "Firefox 121 on Windows 11 x64 en-US",
		// Ja3:           "772,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-34-51-43-13-45-28-65037,29-23-24-25-256-257,0",
		// utls do not support TLS 34 delegated_credentials (34) (IANA) extension at this time.
		// see https://github.com/refraction-networking/utls/issues/274
		Ja3:           "772,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-51-43-13-45-28-65037,29-23-24-25-256-257,0",
		H2fingerpring: "1:65536,4:131072,5:16384|12517377|3:0:0:201,5:0:0:101,7:0:0:1,9:0:7:1,11:0:3:1,13:0:0:241|m,p,a,s",
		Headers: [][]string{
			{"User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0"},
			{"Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"},
			{"Accept-Language", "en-US,en;q=0.5"},
			{"Accept-Encoding", "gzip, deflate, br"},
			{"Cookie", HTTP_HEADER_PLACEHOLDER},
			{"Upgrade-Insecure-Requests", "1"},
			{"Sec-Fetch-Dest", `document`},
			{"Sec-Fetch-Mode", `navigate`},
			{"Sec-Fetch-Site", `none`},
			{"Sec-Fetch-User", `?1`},
			{"te", "trailers"},
		},
	},
}

var (
	// all supported impersonate names
	Impersonates []string
)

func init() {
	for key := range ImpersonateProfiles {
		Impersonates = append(Impersonates, key)
	}
	slices.Sort(Impersonates)
}

func FetchJson(url string, v any, client *http.Client) error {
	res, _, err := FetchUrl(url, client)
	if err != nil {
		return err
	}
	log.Tracef("FetchJson response: len=%d", res.ContentLength)
	body, _ := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	err = json.Unmarshal(body, &v)
	if err != nil {
		log.Tracef("FetchJson failed to unmarshal, response body: %s", string(body))
	}
	return err
}

func FetchUrl(url string, client *http.Client) (*http.Response, http.Header, error) {
	log.Tracef("FetchUrl url=%s", url)
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		if _, ok := err.(net.Error); ok {
			return nil, nil, fmt.Errorf("failed to fetch url: <network error>: %v", err)
		}
		return nil, nil, fmt.Errorf("failed to fetch url: %v", err)
	}
	log.Tracef("FetchUrl response status=%d", res.StatusCode)
	if res.StatusCode != 200 {
		defer res.Body.Close()
		return nil, res.Header, fmt.Errorf("failed to fetch url: status=%d", res.StatusCode)
	}
	return res, res.Header, nil
}

func FetchJsonWithAzuretls(url string, v any, client *azuretls.Session,
	cookie string, ua string, headers [][]string) error {
	res, _, err := FetchUrlWithAzuretls(url, client, cookie, ua, headers)
	if err != nil {
		return err
	}
	log.Tracef("FetchJsonWithAzuretls response: len=%d", res.ContentLength)
	err = json.Unmarshal(res.Body, &v)
	if err != nil {
		log.Tracef("FetchJsonWithAzuretls failed to unmarshal, response body: %s", string(res.Body))
	}
	return err
}

func FetchUrlWithAzuretls(url string, client *azuretls.Session,
	cookie string, ua string, headers [][]string) (*azuretls.Response, http.Header, error) {
	log.Tracef("FetchUrlWithAzuretls url=%s hasCookie=%t", url, cookie != "")
	reqHeaders := GetHttpReqHeaders(headers, cookie, ua)
	res, err := client.Get(url, reqHeaders)
	if err != nil {
		if _, ok := err.(net.Error); ok {
			return nil, nil, fmt.Errorf("failed to fetch url: <network error>: %v", err)
		}
		return nil, nil, fmt.Errorf("failed to fetch url: %v", err)
	}
	log.Tracef("FetchUrlWithAzuretls response status=%d", res.StatusCode)
	if res.StatusCode != 200 {
		return nil, http.Header(res.Header), fmt.Errorf("failed to fetch url: status=%d", res.StatusCode)
	}
	return res, http.Header(res.Header), nil
}

func ParseUrlHostname(urlStr string) string {
	hostname := ""
	url, err := url.Parse(urlStr)
	if err == nil {
		hostname = url.Hostname()
	}
	return hostname
}

func PostUrlForJson(url string, data url.Values, v any, client *http.Client) error {
	if client == nil {
		client = http.DefaultClient
	}
	log.Tracef("PostUrlForJson request url=%s, data=%v", url, data)
	res, err := client.PostForm(url, data)
	if err != nil {
		return err
	}
	log.Tracef("PostUrlForJson response: len=%d", res.ContentLength)
	body, _ := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("PostUrlForJson response error: status=%d", res.StatusCode)
	}
	err = json.Unmarshal(body, &v)
	if err != nil {
		log.Tracef("PostUrlForJson error encountered when unmarshaling: %v", err)
	}
	return err
}

func GetHttpReqHeaders(headers [][]string, cookie string, ua string) azuretls.OrderedHeaders {
	allHeaders := [][]string{}
	effectiveHeaders := [][]string{}
	headerIndexs := map[string]int{}
	allHeaders = append(allHeaders, headers...)
	if cookie != "" {
		allHeaders = append(allHeaders, []string{"Cookie", cookie})
	}
	if ua != "" {
		allHeaders = append(allHeaders, []string{"User-Agent", ua})
	}
	for _, header := range allHeaders {
		headerLowerCase := strings.ToLower(header[0])
		if index, ok := headerIndexs[headerLowerCase]; ok {
			effectiveHeaders[index] = []string{header[0], header[1]}
			if header[1] == "" {
				delete(headerIndexs, headerLowerCase)
			}
		} else if header[1] != "" {
			effectiveHeaders = append(effectiveHeaders, []string{header[0], header[1]})
			headerIndexs[headerLowerCase] = len(effectiveHeaders) - 1
		}
	}
	orderedHeaders := azuretls.OrderedHeaders{}
	for _, header := range effectiveHeaders {
		if header[1] == "" || header[1] == HTTP_HEADER_PLACEHOLDER {
			continue
		}
		orderedHeaders = append(orderedHeaders, header)
	}
	return orderedHeaders
}

func MatchUrlWithHostOrUrl(urlStr string, hostOrUrl string) bool {
	if IsUrl(hostOrUrl) {
		return urlStr == hostOrUrl
	} else {
		urlObj, err := url.Parse(urlStr)
		return err == nil && urlObj.Host == hostOrUrl
	}
}
