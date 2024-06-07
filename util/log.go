package util

import (
	"net/http"
	"net/url"

	"github.com/Noooste/azuretls-client"

	"github.com/sagan/ptool/flags"
	log "github.com/sirupsen/logrus"
)

// Log http request info if dump-headers flag is set.
func LogHttpRequest(req *http.Request) {
	if flags.DumpHeaders {
		log.WithFields(log.Fields{
			"header": req.Header,
			"method": req.Method,
			"url":    req.URL,
		}).Errorf("http request")
	}
}

// Log http response info if dump-headers flag is set.
func LogHttpResponse(res *http.Response, err error) {
	if flags.DumpHeaders {
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
	if flags.DumpHeaders {
		log.WithFields(log.Fields{
			"url":    url,
			"method": "POST",
		}).Errorf("http request")
	}
}

// Log if dump-headers flag is set.
func LogAzureHttpRequest(req *azuretls.Request) {
	if flags.DumpHeaders {
		log.WithFields(log.Fields{
			"header": req.OrderedHeaders,
			"method": req.Method,
			"url":    req.Url,
		}).Errorf("http request")
	}
}

// Log if dump-headers flag is set.
func LogAzureHttpResponse(res *azuretls.Response, err error) {
	if flags.DumpHeaders {
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
