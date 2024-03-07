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
)

// Read a torrent and return it's contents. torrent could be: local filename (e.g.: abc.torrent),
// site torrent id (e.g.: mteam.1234) or url (e.g. https://kp.m-team.cc/details.php?id=488424),
// or "-" to read torrent contents from os.Stdin.
// forceLocal: force treat torrent as local filename. forceRemote: force treat torrent as site torrent id or url.
// ignoreParsingError: ignore torrent parsing error, in which case the returned tinfo may by nil.
// stdin: if not nil, use it as torrent contents when torrent == "-" instead of reading from os.Stdin.
// siteInstance : if torrent is a site torrent, the created corresponding site instance.
// sitename & filename & id : if torrent is a site torrent, the downloaded torrent sitename & filename & id.
func GetTorrentContent(torrent string, defaultSite string,
	forceLocal bool, forceRemote bool, stdin []byte, ignoreParsingError bool) (
	content []byte, tinfo *torrentutil.TorrentMeta, siteInstance site.Site, siteName string,
	filename string, id string, err error) {
	isLocal := !forceRemote && (forceLocal || torrent == "-" ||
		!util.IsUrl(torrent) && strings.HasSuffix(torrent, ".torrent"))
	// site torrent id or url
	if !isLocal {
		if util.IsTorrentUrl(torrent) {
			err = fmt.Errorf("magnet or bt url is NOT supported")
			return
		}
		siteName = defaultSite
		if !util.IsUrl(torrent) {
			if i := strings.Index(torrent, "."); i != -1 {
				siteName = torrent[:i]
			}
		} else if domain := util.GetUrlDomain(torrent); domain == "" {
			err = fmt.Errorf("failed to parse torrent domain")
			return
		} else {
			sitename := ""
			ok := false
			if sitename, ok = domainSiteMap[domain]; !ok {
				if domainSiteMap[domain], err = tpl.GuessSiteByDomain(domain, defaultSite); err == nil {
					sitename = domainSiteMap[domain]
				} else {
					log.Tracef("Failed to find match site for %s: %v", domain, err)
				}
			}
			if sitename == "" {
				log.Tracef("Torrent %s: url does not match any site. will use provided default site", torrent)
			} else {
				siteName = sitename
			}
		}
		if siteName == "" {
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
				if btsite := public.GetSiteByDomain(defaultSite, urlObj.Hostname()); btsite != nil {
					siteName = btsite.Name
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
				var res *azuretls.Response
				var header http.Header
				if res, header, err = util.FetchUrlWithAzuretls(downloadUrl, httpClient, "", "", headers); err != nil {
					err = fmt.Errorf("%s: failed to fetch: %v", torrent, err)
				} else {
					content = res.Body
					filename = util.ExtractFilenameFromHttpHeader(header)
				}
			}
		} else {
			siteInstance, err = site.CreateSite(siteName)
			if err != nil {
				err = fmt.Errorf("failed to create site %s: %v", siteName, err)
				return
			}
			content, filename, id, err = siteInstance.DownloadTorrent(torrent)
		}
	} else {
		if torrent == "-" {
			filename = ""
			if stdin != nil {
				content = stdin
			} else if config.InShell {
				err = fmt.Errorf(`"-" arg can not be used in shell`)
			} else {
				content, err = io.ReadAll(os.Stdin)
			}
		} else if strings.HasSuffix(torrent, ".added") {
			err = fmt.Errorf(".added file is skipped")
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
	if siteName == "" {
		if sitename, err := tpl.GuessSiteByTrackers(tinfo.Trackers, defaultSite); err == nil {
			siteName = sitename
		} else if site := public.GetSiteByDomain(defaultSite, tinfo.Trackers...); site != nil {
			siteName = site.Name
		} else {
			log.Warnf("Failed to find match site for %s by trackers: %v", torrent, err)
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
