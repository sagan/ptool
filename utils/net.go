package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	log "github.com/sirupsen/logrus"
)

var (
	// use latest Chrome stable version on Windows 11
	ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/112.0.0.0 Safari/537.36"
)

func FetchJson(url string, v any, client *http.Client) error {
	res, err := FetchUrl(url, "", client)
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

func FetchUrl(url string, cookie string, client *http.Client) (*http.Response, error) {
	log.Tracef("FetchUrl url=%s hasCookie=%t", url, cookie != "")
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	SetHttpRequestBrowserHeaders(req)
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	if client == nil {
		client = http.DefaultClient
	}
	res, error := client.Do(req)
	if error != nil {
		return nil, fmt.Errorf("failed to fetch url: %v", error)
	}
	log.Tracef("FetchUrl response status=%d", res.StatusCode)
	if res.StatusCode != 200 {
		defer res.Body.Close()
		return nil, fmt.Errorf("failed to fetch url: status=%d", res.StatusCode)
	}
	return res, nil
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
		log.Tracef("PostUrlForJson failed to unmarshal, response body: %s", string(body))
	}
	return err
}

func SetHttpRequestBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", ua)
	req.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("accept-language", "en")
	req.Header.Set("cache-control", "no-cache")
	req.Header.Set("pragma", "no-cache")
}
