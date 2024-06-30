package util

import (
	"mime"
	"net/http"
	"strings"

	"github.com/Noooste/azuretls-client"
	"golang.org/x/exp/slices"

	"github.com/sagan/ptool/flags"
	log "github.com/sirupsen/logrus"
)

var textualMimes = []string{
	"application/json",
	"application/xml",
	"application/x-www-form-urlencoded",
	// "multipart/form-data",
}

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
	if flags.DumpBodies && res != nil {
		logBody("http response body", http.Header(res.Header), res.Body)
	}
}

func logBody(title string, header http.Header, body []byte) {
	maxBinaryBody := 1024
	contentType, isText := getContentType(header)
	if isText {
		log.WithFields(log.Fields{
			"body":        string(body),
			"contentType": contentType,
		}).Errorf(title)
	} else if len(body) <= maxBinaryBody {
		log.WithFields(log.Fields{
			"body":        body,
			"contentType": contentType,
		}).Errorf(title)
	} else {
		log.WithFields(log.Fields{
			"body_start":  body[:1024],
			"length":      len(body),
			"contentType": contentType,
		}).Errorf(title)
	}
}

func LogAzureHttpRequesyBody(req *azuretls.Request, body []byte) {
	if flags.DumpBodies {
		header := http.Header{}
		for _, oh := range req.OrderedHeaders {
			if len(oh) >= 2 {
				header.Set(oh[0], oh[1])
			}
		}
		logBody("http request body", header, body)
	}
}

func LogHttpRequesyBody(req *http.Request, body []byte) {
	if flags.DumpBodies {
		logBody("http request body", req.Header, body)
	}
}

func LogHttpResponseBody(res *http.Response, body []byte) {
	if flags.DumpBodies {
		logBody("http response body", res.Header, body)
	}
}
