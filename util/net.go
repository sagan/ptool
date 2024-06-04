package util

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Noooste/azuretls-client"
	log "github.com/sirupsen/logrus"
)

const (
	// header 占位符。用于保证实际发送 headers 的顺序
	HTTP_HEADER_PLACEHOLDER = "\n"
)

func FetchJson(url string, v any, client *http.Client, header http.Header) error {
	res, _, err := FetchUrl(url, client, header)
	if err != nil {
		return err
	}
	log.Tracef("FetchJson response: len=%d", res.ContentLength)
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	err = json.Unmarshal(body, v)
	if err != nil {
		log.Tracef("FetchJson failed to unmarshal, response body: %s", string(body))
	}
	return err
}

func FetchUrl(url string, client *http.Client, header http.Header) (*http.Response, http.Header, error) {
	log.Tracef("FetchUrl url=%s", url)
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest("GET", url, nil)
	if header != nil {
		req.Header = header
	}
	if err != nil {
		return nil, nil, err
	}
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch url: %w", err)
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

// If http response status is not 200, it return the response, header and an error
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
		return nil, nil, fmt.Errorf("failed to fetch url: %w", err)
	}
	log.Tracef("FetchUrlWithAzuretls response status=%d", res.StatusCode)
	if res.StatusCode != 200 {
		return res, http.Header(res.Header), fmt.Errorf("failed to fetch url: status=%d", res.StatusCode)
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
	defer res.Body.Close()
	log.Tracef("PostUrlForJson response: len=%d", res.ContentLength)
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("PostUrlForJson response error: status=%d", res.StatusCode)
	}
	err = json.Unmarshal(body, v)
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

// Check any error in the err tree is net.Error
func AsNetworkError(err error) bool {
	// errors.As can not be used to against interface. So we must DIY.
	// See https://github.com/golang/go/issues/49177 .
	for err != nil {
		if _, ok := err.(net.Error); ok {
			return true
		}
		err = errors.Unwrap(err)
	}
	return false
}

// Common func for uploading image or other file to public server using post + multipart/form-data request.
func PostUploadFile(client *azuretls.Session, apiUrl string, filename string, fileFieldname string,
	additionalFields url.Values, responseFileUrlField string) (fileUrl string, err error) {
	if fileFieldname == "" {
		fileFieldname = "file"
	}
	if responseFileUrlField == "" {
		responseFileUrlField = "url"
	}
	body := new(bytes.Buffer)
	mp := multipart.NewWriter(body)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
		EscapeQuotes(fileFieldname), EscapeQuotes(filepath.Base(filename))))
	h.Set("Content-Type", mime.TypeByExtension(filepath.Ext(filename)))
	filePartWriter, err := mp.CreatePart(h)
	if err != nil {
		return "", err
	}
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	if _, err = io.Copy(filePartWriter, file); err != nil {
		return "", err
	}
	for field := range additionalFields {
		mp.WriteField(field, additionalFields.Get(field))
	}
	mp.Close()
	req := &azuretls.Request{
		Url:    apiUrl,
		Method: http.MethodPost,
		Body:   body.Bytes(),
		OrderedHeaders: [][]string{
			{"Content-Type", mp.FormDataContentType()},
		},
	}
	log.Tracef("Upload file %q to %s", filename, apiUrl)
	response, err := client.Do(req)
	if err != nil {
		return "", err
	}
	if response.StatusCode != 200 {
		return "", fmt.Errorf("status=%d", response.StatusCode)
	}
	var res map[string]any
	err = json.Unmarshal(response.Body, &res)
	if err != nil || res == nil {
		return "", fmt.Errorf("failed to parse response as json: %w", err)
	}
	if str, ok := res[responseFileUrlField].(string); !ok {
		return "", fmt.Errorf("result %q field is not a string", responseFileUrlField)
	} else {
		return str, nil
	}
}
