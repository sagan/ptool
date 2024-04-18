// Utilities funcs that have side effects.
package helper

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Noooste/azuretls-client"
	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"

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
	ErrGetTorrentUrlIsMagnet                   = fmt.Errorf("magnet or bt url is NOT supported")
	ErrGetTorrentUrlParseFail                  = fmt.Errorf("failed to parse torrent domain")
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
	isLocal = !forceRemote && (forceLocal || torrent == "-" || !util.IsUrl(torrent) && (strings.HasSuffix(
		util.TrimAnySuffix(torrent, constants.ProcessedFilenameSuffixes...), ".torrent")))
	// site torrent id or url
	if !isLocal {
		if util.IsPureTorrentUrl(torrent) {
			err = ErrGetTorrentUrlIsMagnet
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
		} else {
			filename = path.Base(torrent)
			var file *os.File
			if file, err = os.Open(torrent); err == nil {
				defer file.Close()
				var info fs.FileInfo
				if info, err = file.Stat(); err == nil && info.Size() >= constants.BIG_FILE_SIZE {
					fileHeader := make([]byte, 0, constants.FILE_HEADER_CHUNK_SIZE)
					if _, err = io.ReadAtLeast(file, fileHeader, len(fileHeader)); err == nil {
						if util.BytesHasAnyStringPrefix(fileHeader, constants.TorrentFileMagicNumbers...) {
							_, err = file.Seek(0, 0)
						} else {
							err = fmt.Errorf("%s: header is NOT a valid .torrent contents", torrent)
						}
					}
				}
				if err == nil {
					content, err = io.ReadAll(file)
				}
			}
		}
	}
	if err != nil {
		return
	}
	if !util.BytesHasAnyStringPrefix(content, constants.TorrentFileMagicNumbers...) {
		err = fmt.Errorf("%s: is NOT a valid .torrent contents", torrent)
		return
	}
	if tinfo, err = torrentutil.ParseTorrent(content); err != nil {
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

// "*.torrent" => ["a.torrent", "b.torrent"...].
// Return filestr untouched if it does not contains wildcard char.
// Windows cmd / powershell 均不支持命令行 *.torrent 参数扩展。必须应用自己实现。做个简易版的
func GetWildcardFilenames(filestr string) []string {
	if !strings.ContainsAny(filestr, "*") {
		return nil
	}
	dir := filepath.Dir(filestr)
	name := filepath.Base(filestr)
	ext := filepath.Ext(name)
	if ext != "" {
		name = name[:len(name)-len(ext)]
	}
	prefix := ""
	suffix := ""
	exact := ""
	index := strings.Index(name, "*")
	if index != -1 {
		prefix = name[:index]
		suffix = name[index+1:]
	} else {
		exact = name
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	filenames := []string{}
	for _, entry := range entries {
		entryName := entry.Name()
		entryExt := filepath.Ext(entryName)
		if ext != "" {
			if entryExt == "" || (entryExt != ext && ext != ".*") {
				continue
			}
			entryName = entryName[:len(entryName)-len(entryExt)]
		}
		if exact != "" && entryName != exact {
			continue
		}
		if prefix != "" && !strings.HasPrefix(entryName, prefix) {
			continue
		}
		if suffix != "" && !strings.HasSuffix(entryName, suffix) {
			continue
		}
		filenames = append(filenames, dir+"/"+entry.Name())
	}
	return filenames
}

func ParseFilenameArgs(args ...string) []string {
	names := []string{}
	for _, arg := range args {
		filenames := GetWildcardFilenames(arg)
		if filenames == nil {
			names = append(names, arg)
		} else {
			names = append(names, filenames...)
		}
	}
	return names
}

// Ask user to confirm an (dangerous) action via typing yes in tty
func AskYesNoConfirm(prompt string) bool {
	if prompt == "" {
		prompt = "Will do the action"
	}
	fmt.Fprintf(os.Stderr, "%s, are you sure? (yes/no): ", prompt)
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintf(os.Stderr, `Abort due to stdin is NOT tty. Use a proper flag (like "--force") to skip the prompt`+"\n")
		return false
	}
	for {
		input := ""
		fmt.Scanf("%s\n", &input)
		switch input {
		case "yes", "YES", "Yes":
			return true
		case "n", "N", "no", "NO", "No":
			return false
		default:
			if len(input) > 0 {
				fmt.Fprintf(os.Stderr, "Respond with yes or no (Or use Ctrl+C to abort): ")
			} else {
				return false
			}
		}
	}
}

// Parse torrents list from args.
// A single "-" args will make it read torrents list from stdin instead,
// unless stdin contents is a valid .torrent file, in which case returned torrents is ["-"]
// and stdin contents returned as stdinTorrentContents.
func ParseTorrentsFromArgs(args []string) (torrents []string, stdinTorrentContents []byte, err error) {
	stdinTorrentContents = []byte{}
	torrents = ParseFilenameArgs(args...)
	if len(torrents) == 1 && torrents[0] == "-" {
		if config.InShell {
			err = fmt.Errorf(`"-" arg can not be used in shell`)
		} else if stdin, _err := io.ReadAll(os.Stdin); _err != nil {
			err = fmt.Errorf("failed to read stdin: %v", _err)
		} else if util.BytesHasAnyStringPrefix(stdin, constants.TorrentFileMagicNumbers...) {
			stdinTorrentContents = stdin
		} else if data, _err := shlex.Split(string(stdin)); _err != nil {
			err = fmt.Errorf("failed to parse stdin to tokens: %v", _err)
		} else {
			torrents = data
		}
	} else if slices.Contains(torrents, "-") {
		err = fmt.Errorf(`"-" arg can NOT be mixed up with others`)
	}
	return
}

// Parse info-hash list from args. If args is a single "-", read list from stdin instead.
// Specially, it returns an error is args is empty.
func ParseInfoHashesFromArgs(args []string) (infoHashes []string, err error) {
	if len(args) == 0 {
		err = fmt.Errorf("you must provide at least a arg or filter flag")
		return
	}
	infoHashes = args
	if len(infoHashes) == 1 && infoHashes[0] == "-" {
		if data, _err := ReadArgsFromStdin(); _err != nil {
			err = fmt.Errorf("failed to parse stdin to info hashes: %v", _err)
		} else if len(data) == 0 {
			infoHashes = nil
		} else {
			infoHashes = data
		}
	} else if slices.Contains(infoHashes, "-") {
		err = fmt.Errorf(`"-" arg can NOT be mixed up with others`)
	}
	return
}
