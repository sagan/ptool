package util

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/Noooste/azuretls-client"
	log "github.com/sirupsen/logrus"
)

const (
	// header 占位符。用于保证实际发送 headers 的顺序
	HTTP_HEADER_PLACEHOLDER = "\n"
)

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
	res, err := client.Do(&azuretls.Request{
		Method:   http.MethodGet,
		Url:      url,
		NoCookie: true, // disable azuretls internal cookie jar
	}, reqHeaders)
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

// Extract filename from http response "Content-Disposition: attachment; filename=..." header
func ExtractFilenameFromHttpHeader(header http.Header) (filename string) {
	if _, params, err := mime.ParseMediaType(header.Get("content-disposition")); err == nil {
		unescapedFilename, err := url.QueryUnescape(params["filename"])
		if err == nil {
			filename = unescapedFilename
		}
	}
	return
}
