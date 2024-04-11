package helper

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/Noooste/azuretls-client"
	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/site/public"
	"github.com/sagan/ptool/site/tpl"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/torrentutil"
)

var (
	domainSiteMap = map[string]string{}
	// The unix timestamp (miliseconds) last time download torrent from a site
	siteDownloadTimeMap                        = map[string]int64{}
	ErrGetTorrentUrlIsMegnet                   = fmt.Errorf("magnet or bt url is NOT supported")
	ErrGetTorrentUrlParseFail                  = fmt.Errorf("failed to parse torrent domain")
	ErrGetTorrentSkipped                       = fmt.Errorf(".added or .failed file is skipped")
	ErrGetTorrentStdoutOutputNotSupportInShell = fmt.Errorf(`"-" arg can not be used in shell`)
)

// Read a torrent and return it's contents. torrent could be: local filename (e.g.: abc.torrent),
// site torrent id (e.g.: mteam.1234) or url (e.g. https://kp.m-team.cc/details.php?id=488424),
// or "-" to read torrent contents from os.Stdin.
// Params:
// forceLocal: force treat torrent as local filename. forceRemote: force treat torrent as site torrent id or url.
// ignoreParsingError: ignore torrent parsing error, in which case the returned tinfo may by nil.
// stdin: if not nil, use it as torrent contents when torrent == "-" instead of reading from os.Stdin.
// beforeDownload: a optional func that be called before downloading each torrent from remote,
// if the func return a non-nil error, will NOT do download and instead return that err.
// Return:
// siteInstance : if torrent is a site torrent, the created corresponding site instance.
// sitename & filename & id : if torrent is a site torrent, the downloaded torrent sitename & filename & id.
// isLocal: whether torrent is a local or remote torrent.
func GetTorrentContent(torrent string, defaultSite string,
	forceLocal bool, forceRemote bool, stdin []byte, ignoreParsingError bool,
	beforeDownload func(sitename string, id string) error) (
	content []byte, tinfo *torrentutil.TorrentMeta, siteInstance site.Site, sitename string,
	filename string, id string, isLocal bool, err error) {
	isLocal = !forceRemote && (forceLocal || torrent == "-" ||
		!util.IsUrl(torrent) && strings.HasSuffix(torrent, ".torrent"))
	// site torrent id or url
	if !isLocal {
		if util.IsPureTorrentUrl(torrent) {
			err = ErrGetTorrentUrlIsMegnet
			return
		}
		sitename = defaultSite
		if !util.IsUrl(torrent) {
			if i := strings.Index(torrent, "."); i != -1 {
				sitename = torrent[:i]
				id = torrent[i+1:]
			}
		} else if domain := util.GetUrlDomain(torrent); domain == "" {
			err = ErrGetTorrentUrlParseFail
			return
		} else {
			_sitename := ""
			ok := false
			if _sitename, ok = domainSiteMap[domain]; !ok {
				if domainSiteMap[domain], err = tpl.GuessSiteByDomain(domain, defaultSite); err == nil {
					_sitename = domainSiteMap[domain]
				} else {
					log.Tracef("Failed to find match site for %s: %v", domain, err)
				}
			}
			if _sitename == "" {
				log.Tracef("Torrent %s: url does not match any site. will use provided default site", torrent)
			} else {
				sitename = _sitename
			}
		}
		if sitename == "" {
			log.Tracef("%s: no site found, try to download torrent from url directly", torrent)
			var urlObj *url.URL
			var httpClient *azuretls.Session
			var headers [][]string
			if urlObj, err = url.Parse(torrent); err != nil {
				err = fmt.Errorf("%s: no site found and url is invalid", torrent)
			} else if httpClient, headers, err = site.CreateSiteHttpClient(&config.SiteConfigStruct{},
				config.Get()); err != nil {
				err = fmt.Errorf("%s: no site found and failed to create public http client", torrent)
			} else {
				downloadUrl := torrent
				btsite := public.GetSiteByDomain(defaultSite, urlObj.Hostname())
				if btsite != nil {
					sitename = btsite.Name
					if btsite.TorrentUrlIdRegexp != nil {
						if m := btsite.TorrentUrlIdRegexp.FindStringSubmatch(torrent); m != nil {
							id = m[btsite.TorrentUrlIdRegexp.SubexpIndex("id")]
							downloadUrl = btsite.TorrentDownloadUrl
							downloadUrl = strings.ReplaceAll(downloadUrl, "{{id}}", id)
							downloadUrl = strings.ReplaceAll(downloadUrl, "{{domain}}", btsite.Domains[0])
							downloadUrl = strings.ReplaceAll(downloadUrl, "{{origin}}", urlObj.Scheme+"://"+urlObj.Host)
						}
					}
					log.Tracef("found public bittorrent site %s torrent download url %s", btsite.Name, downloadUrl)
				}
				if beforeDownload != nil {
					if err = beforeDownload(sitename, id); err != nil {
						return
					}
				}
				if btsite != nil && btsite.TorrentDownloadInterval > 0 && siteDownloadTimeMap[btsite.Name] > 0 {
					elapsed := time.Now().UnixMilli() - siteDownloadTimeMap[btsite.Name]
					if elapsed >= 0 && elapsed < btsite.TorrentDownloadInterval {
						time.Sleep(time.Millisecond * time.Duration(btsite.TorrentDownloadInterval-elapsed))
					}
				}
				var res *azuretls.Response
				var header http.Header
				if res, header, err = util.FetchUrlWithAzuretls(downloadUrl, httpClient, "", "", headers); err != nil {
					err = fmt.Errorf("%s: failed to fetch: %v", torrent, err)
				} else {
					content = res.Body
					filename = util.ExtractFilenameFromHttpHeader(header)
				}
				if sitename != "" {
					siteDownloadTimeMap[sitename] = time.Now().UnixMilli()
				}
			}
		} else {
			if beforeDownload != nil {
				if err = beforeDownload(sitename, id); err != nil {
					return
				}
			}
			siteInstance, err = site.CreateSite(sitename)
			if err != nil {
				err = fmt.Errorf("failed to create site %s: %v", sitename, err)
				return
			}
			content, filename, id, err = siteInstance.DownloadTorrent(torrent)
			siteDownloadTimeMap[sitename] = time.Now().UnixMilli()
		}
	} else {
		if torrent == "-" {
			filename = ""
			if stdin != nil {
				content = stdin
			} else if config.InShell {
				err = ErrGetTorrentStdoutOutputNotSupportInShell
			} else {
				content, err = io.ReadAll(os.Stdin)
			}
		} else if strings.HasSuffix(torrent, ".added") || strings.HasSuffix(torrent, ".failed") {
			err = ErrGetTorrentSkipped
		} else {
			filename = path.Base(torrent)
			content, err = os.ReadFile(torrent)
		}
	}
	if err != nil {
		return
	}
	if !bytes.HasPrefix(content, []byte(constants.TORRENT_FILE_MAGIC_NUMBER)) {
		err = fmt.Errorf("%s: content is NOT a valid .torrent file", torrent)
		return
	}
	if tinfo, err = torrentutil.ParseTorrent(content, 99); err != nil {
		msg := fmt.Sprintf("%s: failed to parse torrent: %v", torrent, err)
		if ignoreParsingError {
			log.Debugf(msg)
			err = nil
		} else {
			err = fmt.Errorf(msg)
		}
		return
	}
	if sitename == "" {
		if _sitename, err := tpl.GuessSiteByTrackers(tinfo.Trackers, defaultSite); err != nil {
			log.Warnf("Failed to find match site for %s by trackers: %v", torrent, err)
		} else if _sitename != "" {
			sitename = _sitename
		} else if site := public.GetSiteByDomain(defaultSite, tinfo.Trackers...); site != nil {
			sitename = site.Name
		}
	}
	return
}

// Read whitespace splitted tokens from stdin
func ReadArgsFromStdin() ([]string, error) {
	if config.InShell {
		return nil, fmt.Errorf(`can NOT read args from stdin in shell mode`)
	} else if stdin, err := io.ReadAll(os.Stdin); err != nil {
		return nil, fmt.Errorf("failed to read stdin: %v", err)
	} else if data, err := shlex.Split(string(stdin)); err != nil {
		return nil, fmt.Errorf("failed to parse stdin to tokens: %v", err)
	} else {
		return data, nil
	}
}
