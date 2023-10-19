package util

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	log "github.com/sirupsen/logrus"
)

var (
	// from https://scrapfly.io/web-scraping-tools/ja3-fingerprint
	// must generate it without the "TLS Session has been resurected" warning
	CHROME_JA3 = "772,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,65281-5-11-17513-0-27-13-43-45-51-35-18-16-23-10,29-23-24,0"
	// 最新稳定版 Chrome (en-US) 在 Windows 11 x64 环境下访问网页的默认请求 headers
	CHROME_HTTP_REQUEST_HEADERS = map[string]string{
		"Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		// "Accept-Encoding":           "gzip, deflate, br",
		"Accept-Language":           "en-US,en;q=0.9",
		"Cache-Control":             "no-cache",
		"Connection":                "keep-alive",
		"Pragma":                    "no-cache",
		"Sec-Ch-Ua":                 `"Chromium";v="118", "Google Chrome";v="118", "Not=A?Brand";v="99"`,
		"Sec-Ch-Ua-Mobile":          `?0`,
		"Sec-Ch-Ua-Platform":        `"Windows"`,
		"Sec-Fetch-Dest":            `document`,
		"Sec-Fetch-Mode":            `navigate`,
		"Sec-Fetch-Site":            `same-origin`,
		"Sec-Fetch-User":            `?1`,
		"Upgrade-Insecure-Requests": "1",
		"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
	}
)

func FetchJson(url string, v any, client *http.Client,
	cookie string, ua string, otherHeaders map[string]string) error {
	res, _, err := FetchUrl(url, client, cookie, ua, otherHeaders)
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

func FetchUrl(url string, client *http.Client,
	cookie string, ua string, otherHeaders map[string]string) (*http.Response, http.Header, error) {
	log.Tracef("FetchUrl url=%s hasCookie=%t", url, cookie != "")
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}
	SetHttpRequestBrowserHeaders(req, ua)
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	for header, value := range otherHeaders {
		req.Header.Set(header, value)
	}
	if client == nil {
		client = http.DefaultClient
	}
	res, error := client.Do(req)
	if error != nil {
		return nil, nil, fmt.Errorf("failed to fetch url: %v", error)
	}
	log.Tracef("FetchUrl response status=%d", res.StatusCode)
	if res.StatusCode != 200 {
		defer res.Body.Close()
		return nil, res.Header, fmt.Errorf("failed to fetch url: status=%d", res.StatusCode)
	}
	return res, res.Header, nil
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

func SetHttpRequestBrowserHeaders(req *http.Request, ua string) {
	for key, value := range CHROME_HTTP_REQUEST_HEADERS {
		req.Header.Set(key, value)
	}
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
}

func MatchUrlWithHostOrUrl(urlStr string, hostOrUrl string) bool {
	if IsUrl(hostOrUrl) {
		return urlStr == hostOrUrl
	} else {
		urlObj, err := url.Parse(urlStr)
		return err == nil && urlObj.Host == hostOrUrl
	}
}
