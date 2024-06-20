package util

import (
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/Noooste/azuretls-client"
	"golang.org/x/exp/slices"

	"github.com/sagan/ptool/flags"
	log "github.com/sirupsen/logrus"
)

var textualMimes = []string{"application/json", "application/xml"}

func getContentType(header http.Header) (contentType string, isText bool) {
	contentType, _, _ = mime.ParseMediaType(header.Get("Content-Type"))
	isText = slices.Contains(textualMimes, contentType) || strings.HasPrefix(contentType, "text/")
	return
}

// Log http request info if dump-headers flag is set.
func LogHttpRequest(req *http.Request) {
	if flags.DumpHeaders || flags.DumpBodies {
		log.WithFields(log.Fields{
			"header": req.Header,
			"method": req.Method,
			"url":    req.URL,
		}).Errorf("http request")
	}
}

// Log http response info if dump-headers flag is set.
func LogHttpResponse(res *http.Response, err error) {
	if flags.DumpHeaders || flags.DumpBodies {
		if res != nil {
			log.WithFields(log.Fields{
				"header": res.Header,
				"status": res.StatusCode,
				"error":  err,
			}).Errorf("http response")
		} else {
			log.WithFields(log.Fields{
				"error": err,
			}).Errorf("http response")
		}
	}
}

// Log if dump-headers flag is set.
func LogHttpPostFormRequest(url string, data url.Values) {
	if flags.DumpHeaders || flags.DumpBodies {
		log.WithFields(log.Fields{
			"url":    url,
			"method": "POST",
		}).Errorf("http request")
	}
}

// Log if dump-headers flag is set.
func LogAzureHttpRequest(req *azuretls.Request) {
	if flags.DumpHeaders || flags.DumpBodies {
		log.WithFields(log.Fields{
			"header": req.OrderedHeaders,
			"method": req.Method,
			"url":    req.Url,
		}).Errorf("http request")
	}
}

// Log if dump-headers flag is set.
func LogAzureHttpResponse(res *azuretls.Response, err error) {
	if flags.DumpHeaders || flags.DumpBodies {
		if res != nil {
			log.WithFields(log.Fields{
				"header": res.Header,
				"status": res.StatusCode,
				"error":  err,
			}).Errorf("http response")
		} else {
			log.WithFields(log.Fields{
				"error": err,
			}).Errorf("http response")
		}
	}
	if flags.DumpBodies {
		contentType, isText := getContentType(http.Header(res.Header))
		if isText {
			log.WithFields(log.Fields{
				"body":        string(res.Body),
				"contentType": contentType,
			}).Errorf("http response body")
		} else {
			log.WithFields(log.Fields{
				"body":        res.Body,
				"contentType": contentType,
			}).Errorf("http response body")
		}
	}
}

func LogHttpResponseBody(res *http.Response, body []byte) {
	if flags.DumpBodies {
		contentType, isText := getContentType(res.Header)
		if isText {
			log.WithFields(log.Fields{
				"body":        string(body),
				"contentType": contentType,
			}).Errorf("http response body")
		} else {
			log.WithFields(log.Fields{
				"body":        body,
				"contentType": contentType,
			}).Errorf("http response body")
		}
	}
}
