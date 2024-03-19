package reseed

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf16"

	socketio "github.com/googollee/go-socket.io"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
	log "github.com/sirupsen/logrus"
)

// Reseed API backend: https://github.com/tongyifan/Reseed-backend , it's a sock.io server,
// with the main websocket API and some additional RESTful APIs.
// All APIs (except "login" API) use token authorization in header: "Authorization: Bearar <token>",
// To acquire token: POST https://reseed-api.tongyifan.me/login with username & password,
// receive json {msg, success: true, token}. token is ephemeral, expires in 1 day.
// Note for websocket API, if token does NOT exists, server will return 500 error when connecting;
// however, if token exists but is expired or invalid, server will just hang (do NOT response to any request).
// The websocket API uses sock.io events:
// "file" event: client -> server.
// "reseed result" event: server -> client.
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

type ReseedResultSite struct {
	// reseed torrent id
	Id int64 `json:"id,omitempty"`
	// comma-separated site torrent ids, e.g.: NexusHD-123456,HDU-23456,TJUPT-34567
	Sites string `json:"sites,omitempty"`
}

type Torrent struct {
	Id       string
	ReseedId string
	SavePath string
	Filename string
	Flag     string
}

// the 'reseed result' event returned by Reseed backend
// see https://github.com/tongyifan/Reseed-backend/blob/890dfcb20b98684bf315c8c9f5352c062ae93166/views/reseed.py#L57
type ReseedResult struct {
	Name       string             `json:"name,omitempty"`
	CmpSuccess []ReseedResultSite `json:"cmp_success,omitempty"`
	CmpWarning []ReseedResultSite `json:"cmp_warning,omitempty"`
}

func (t *Torrent) String() string {
	return t.Id
}

// It's performance is terrible, but who cares?
func (f File) MarshalJSON() ([]byte, error) {
	if bytes, err := json.Marshal(map[string]any(f)); err != nil {
		return nil, err
	} else {
		return []byte(Escape2Anscii(string(bytes))), nil
	}
}

// Escape all unicode (non-ASCII) characters to '\uXXXX' format.
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

// Request Reseed API and return xseed torrents (full match (success) & partial match (warning) results)
// found by Reseed backend.
func GetReseedTorrents(username string, password string, sites []*config.SiteConfigStruct, timeout int64,
	savePath ...string) (results []*Torrent, results2 []*Torrent, err error) {
	file, savePathMap, err := scan(savePath...)
	if err != nil {
		err = fmt.Errorf("failed to scan savePath(s): %v", err)
		return
	}
	if len(file) == 0 {
		log.Debugf("All savePath does NOT has any contents")
		return
	}
	token, err := Login(username, password)
	if err != nil {
		err = fmt.Errorf("failed to login to reseed server: %v", err)
		return
	}
	reseedSites, err := GetSites(token)
	if err != nil {
		err = fmt.Errorf("failed to get reseed sites: %v", err)
		return
	}
	reseed2LocalMap := GenerateReseed2LocalSiteMap(reseedSites, sites)
	client, err := socketio.NewClient(RESEED_API, nil)
	if err != nil {
		err = fmt.Errorf("failed to create sock.io client: %v", err)
		return
	}
	// must provide a "reply" event listener
	client.OnEvent("reply", func(s socketio.Conn, msg string) {
		// log.Println("Receive Message /reply: ", "reply", msg)
	})
	header := http.Header{}
	if token != "" {
		header.Set("Authorization", "Bearar "+token)
	}
	if err = client.Connect(header); err != nil {
		err = fmt.Errorf("failed to connect to reseed backend: %v", err)
		return
	}
	timeoutPeriod := time.Second * time.Duration(timeout)
	timeoutTicker := time.NewTicker(timeoutPeriod)
	chResult := make(chan *ReseedResult, 1)
	chErr := make(chan error, 1)
	client.OnEvent("reseed result", func(conn socketio.Conn, message *ReseedResult) {
		log.Tracef("reseed result: %v", message)
		chResult <- message
	})
	client.OnError(func(c socketio.Conn, err error) {
		log.Tracef("Server error: %v", err)
		chErr <- err
	})
	go client.Emit("file", file)
	cntResult := 0
loop:
	for {
		select {
		case result := <-chResult:
			log.Tracef("reseed result: %v", result)
			cntResult++
			results = append(results, parseReseedResult(reseed2LocalMap, savePathMap, result.Name, result.CmpSuccess)...)
			results2 = append(results2, parseReseedResult(reseed2LocalMap, savePathMap, result.Name, result.CmpWarning)...)
			timeoutTicker.Reset(timeoutPeriod)
			if cntResult == len(file) {
				break loop
			}
		case e := <-chErr:
			err = e
			break loop
		case <-timeoutTicker.C: // the websocket API has no "end" event.
			break loop
		}
	}
	client.Close()
	timeoutTicker.Stop()
	if cntResult == 0 {
		log.Debugf("server did not return any response")
	}
	for _, torrent := range results2 {
		torrent.Flag = "warning"
	}
	return
}

// Scan top-level "Download" dirs and generate Reseed "file" request payload
func scan(dirs ...string) (file File, savePathMap map[string]string, err error) {
	file = File{}
	savePathMap = map[string]string{}
	for _, dir := range dirs {
		err = filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				if path != dir && info.IsDir() {
					log.Errorf("Failed to access %s (dir=%t, err=%v), skip it", info.Name(), info.IsDir(), err)
					return filepath.SkipDir
				}
				return err
			}
			if !info.Mode().IsRegular() && !info.IsDir() {
				return nil
			}
			relpath, err := filepath.Rel(dir, path)
			if err != nil || relpath == "" || relpath == "." {
				return nil // just ignore it
			}
			relpath = strings.ReplaceAll(relpath, `/`, `\`)
			inTopLevelDir := !strings.Contains(relpath, `\`)
			if info.IsDir() {
				if inTopLevelDir && (strings.HasPrefix(info.Name(), ".") || strings.HasPrefix(info.Name(), "$")) {
					return filepath.SkipDir
				}
			} else {
				if strings.HasSuffix(path, ".torrent") || strings.HasSuffix(path, ".added") {
					return nil
				}
			}
			if index := strings.Index(relpath, `\`); index == -1 { // top-level item
				savePathMap[relpath] = dir
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
	err := util.FetchJson(RESEED_API+"sites_info", &sites, nil, http.Header{
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

type ReseedLoginResult struct {
	Msg     string
	Success bool
	Token   string
}

func Login(username string, password string) (token string, err error) {
	data := url.Values{
		"username": []string{username},
		"password": []string{password},
	}
	var result *ReseedLoginResult
	if err = util.PostUrlForJson(RESEED_API+"login", data, &result, nil); err != nil {
		return "", fmt.Errorf("failed to login: %v", err)
	}
	if !result.Success {
		return "", fmt.Errorf("failed login: %s", result.Msg)
	}
	return result.Token, nil
}

func parseReseedResult(reseed2LocalMap map[string]string, savePathMap map[string]string, name string,
	sites []ReseedResultSite) (results []*Torrent) {
	for _, successResult := range sites {
		torrents := util.SplitCsv(successResult.Sites)
		for _, torrent := range torrents {
			id := ""
			info := strings.Split(torrent, "-")
			if len(info) == 2 && reseed2LocalMap[info[0]] != "" {
				id = reseed2LocalMap[info[0]] + "." + info[1]
			}
			results = append(results, &Torrent{
				Id:       id,
				ReseedId: torrent,
				SavePath: savePathMap[name],
				Filename: name,
			})
		}
	}
	return
}
