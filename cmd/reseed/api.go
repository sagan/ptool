package reseed

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf16"

	socketio "github.com/googollee/go-socket.io"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
	log "github.com/sirupsen/logrus"
)

// backend: https://github.com/tongyifan/Reseed-backend .
// It's using a sock.io server as API,
// Note the websocket API ONLY accept frames that is pure ASCII,
// all unicode characters in JSON must be encoded in '\uXXXX' format.
const RESEED_API = "https://reseed-api.tongyifan.me/"

type Site struct {
	BaseUrl string `json:"base_url,omitempty"` // e.g.: "https://hdtime.org/"
	Name    string `json:"name,omitempty"`     // e.g.: "HDTIME"
	// we do not need below fields:
	// _enable
	// _passkey
}

// the "file" event payload of Reseed sock.io API
type File map[string]any

// the 'reseed result' event returned by Reseed backend
// see https://github.com/tongyifan/Reseed-backend/blob/890dfcb20b98684bf315c8c9f5352c062ae93166/views/reseed.py#L57
type ReseedResult struct {
	Name       string `json:"name,omitempty"`
	CmpSuccess struct {
		Id int64 `json:"id,omitempty"`
		// comma-separated site torrent ids, e.g.: NexusHD-123456,HDU-23456,TJUPT-34567
		Sites string `json:"sites,omitempty"`
	} `json:"cmp_success,omitempty"`
	CmpWarning []string `json:"cmp_warning,omitempty"`
}

// It's performance is terrible, but who cares?
func (f File) MarshalJSON() ([]byte, error) {
	if bytes, err := json.Marshal(map[string]any(f)); err != nil {
		return nil, err
	} else {
		return []byte(Escape2Anscii(string(bytes))), nil
	}
}

func Escape2Anscii(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if r > 0xFFFF {
			r1, r2 := utf16.EncodeRune(r)
			sb.WriteString(fmt.Sprintf("\\u%04X", r1))
			sb.WriteString(fmt.Sprintf("\\u%04X", r2))
		} else if r > 0x7F {
			sb.WriteString(fmt.Sprintf("\\u%04X", r))
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// return xseed torrent ids found by reseed backend.
func GetReseedTorrents(token string, savePath ...string) ([]string, error) {
	client, _ := socketio.NewClient(RESEED_API, nil)
	header := http.Header{}
	header.Set("Authorization", "Bearar "+token)
	if err := client.Connect(header); err != nil {
		return nil, fmt.Errorf("failed to connect to reseed backend: %v", err)
	}
	file, err := scan(savePath...)
	if err != nil {
		return nil, fmt.Errorf("failed to scan savePath(s): %v", err)
	}
	client.Emit("file", file)
	client.OnEvent("reseed result", func(conn socketio.Conn, message any) {
		log.Tracef("reseed result: %v", message)
	})
	client.OnError(func(c socketio.Conn, err error) {
		log.Tracef("Server error: %v", err)
	})
	return nil, nil
}

// Scan top-level "Download" dirs and generate Reseed "file" request payload
func scan(dirs ...string) (file File, err error) {
	file = File{}
	for _, dir := range dirs {
		err = filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() && !info.IsDir() {
				return nil
			}
			if !info.IsDir() && (strings.HasSuffix(path, ".torrent") || strings.HasSuffix(path, ".added")) {
				return nil
			}
			relpath, err := filepath.Rel(dir, path)
			if err != nil || relpath == "" || relpath == "." {
				return nil // just ignore it
			}
			relpath = strings.ReplaceAll(relpath, `/`, `\`)
			if index := strings.Index(relpath, `\`); index == -1 {
				if info.IsDir() {
					file[relpath] = map[string]any{}
				} else {
					file[relpath] = info.Size()
				}
			} else {
				dirFile := file[relpath[:index]].(map[string]any)
				dirFile[relpath[index+1:]] = info.Size()
			}
			return nil
		})
		if err != nil {
			return
		}
	}
	return
}

// https://reseed-api.tongyifan.me/sites_info
func GetSites(token string) ([]Site, error) {
	var sites []Site
	err := util.FetchJson(util.ParseRelativeUrl("sites_info", RESEED_API), &sites, nil, http.Header{
		"Authorization": []string{"Bearar " + token},
	})
	if err != nil {
		return nil, err
	}
	return sites, nil
}

// return ReseedSite => localSiteName map
func GenerateReseed2LocalSiteMap(reseedSites []Site,
	localSites []*config.SiteConfigStruct) map[string]string {
	iyuu2LocalSiteMap := map[string]string{} // iyuu sid => local site name
	for _, reseedSite := range reseedSites {
		localSite := util.FindInSlice(localSites, func(siteConfig *config.SiteConfigStruct) bool {
			reseedSiteName := strings.ToLower(reseedSite.Name)
			if siteConfig.Url != "" && siteConfig.Url == reseedSite.BaseUrl {
				return true
			}
			regInfo := site.GetConfigSiteReginfo(siteConfig.GetName())
			return regInfo != nil && (regInfo.Name == reseedSiteName || slices.Index(regInfo.Aliases, reseedSiteName) != -1)
		})
		if localSite != nil {
			iyuu2LocalSiteMap[reseedSite.Name] = (*localSite).GetName()
		}
	}
	return iyuu2LocalSiteMap
}
