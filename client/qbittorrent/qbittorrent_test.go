package qbittorrent

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/sagan/ptool/config"
)

func TestAPIPostAcceptsNoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/auth/login" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	qbclient := &Client{
		ClientConfig: &config.ClientConfigStruct{
			Url: server.URL + "/",
		},
		HttpClient: server.Client(),
	}

	if err := qbclient.apiPost("api/v2/auth/login", url.Values{}); err != nil {
		t.Fatalf("apiPost returned error for 204: %v", err)
	}
}

func TestAPIPostRejectsUnsuccessfulStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	qbclient := &Client{
		ClientConfig: &config.ClientConfigStruct{
			Url: server.URL + "/",
		},
		HttpClient: server.Client(),
	}

	err := qbclient.apiPost("api/v2/auth/login", url.Values{})
	if err == nil || !strings.Contains(err.Error(), "status=403") {
		t.Fatalf("apiPost error = %v, want status=403", err)
	}
}

func TestAPIPostRejectsFailsBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Fails."))
	}))
	defer server.Close()

	qbclient := &Client{
		ClientConfig: &config.ClientConfigStruct{
			Url: server.URL + "/",
		},
		HttpClient: server.Client(),
	}

	err := qbclient.apiPost("api/v2/auth/login", url.Values{})
	if err == nil || !strings.Contains(err.Error(), "Fails") {
		t.Fatalf("apiPost error = %v, want Fails error", err)
	}
}
