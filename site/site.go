package site

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Noooste/azuretls-client"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"

	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/jinja"
	"github.com/sagan/ptool/util"
	"github.com/sagan/ptool/util/crypto"
	"github.com/sagan/ptool/util/impersonateutil"
)

// @todo: considering changing it to interface
type Torrent struct {
	Name               string
	Description        string
	Id                 string // optional torrent id in the site
	InfoHash           string
	DownloadUrl        string
	DownloadMultiplier float64
	UploadMultiplier   float64
	DiscountEndTime    int64
	Time               int64 // torrent unix timestamp (seconds)
	Size               int64
	IsSizeAccurate     bool
	Seeders            int64
	Leechers           int64
	Snatched           int64
	HasHnR             bool     // true if has any type of HR
	IsActive           bool     // true if torrent is or had ever been downloaded / seeding
	IsCurrentActive    bool     // true if torrent is currently downloading / seeding. If true, so will be IsActive
	Paid               bool     // "付费"种子: (第一次)下载或汇报种子时扣除魔力/积分
	Bought             bool     // 适用于付费种子：已购买
	Neutral            bool     // 中性种子：不计算上传、下载、做种魔力
	Tags               []string // labels, e.g. category and other meta infos.
}

type Status struct {
	UserName            string
	UserDownloaded      int64
	UserUploaded        int64
	TorrentsSeedingCnt  int64
	TorrentsLeechingCnt int64
}

type Site interface {
	GetName() string
	// default sent http request headers
	GetDefaultHttpHeaders() [][]string
	GetSiteConfig() *config.SiteConfigStruct
	// download torrent by id (e.g. 12345), sitename.id (e.g. mteam.12345),
	// or absolute download url (e.g. https://kp.m-team.cc/download.php?id=12345).
	DownloadTorrent(url string) (content []byte, filename string, id string, err error)
	// download torrent by torrent id (e.g. "12345")
	DownloadTorrentById(id string) (content []byte, filename string, err error)
	GetLatestTorrents(full bool) ([]*Torrent, error)
	// sort: size|name|none(or "")
	GetAllTorrents(sort string, desc bool, pageMarker string, baseUrl string) (
		torrents []*Torrent, nextPageMarker string, err error)
	// can use "%s" as keyword placeholder in baseUrl
	SearchTorrents(keyword string, baseUrl string) ([]*Torrent, error)
	// Publish (upload) new torrent to site, return uploaded torrent id
	// Some keys in metadata should be handled specially:
	// If metadata contains "_dryrun", use dry run mode;
	PublishTorrent(contents []byte, metadata url.Values) (id string, err error)
	GetStatus() (*Status, error)
	PurgeCache()
}

type RegInfo struct {
	Name    string
	Aliases []string
	Creator func(string, *config.SiteConfigStruct, *config.ConfigStruct) (Site, error)
}

type SiteCreator func(*RegInfo) (Site, error)

var (
	// Error that indicates the feature is not implemented in current site.
	ErrUnimplemented = fmt.Errorf("not implemented yet")
)

var (
	registryMap  = map[string]*RegInfo{}
	sites        = map[string]Site{}
	siteSessions = map[string]*azuretls.Session{}
	mu           sync.Mutex
)

func (ss *Status) Print(f io.Writer, name string, additionalInfo string) {
	fmt.Printf(constants.STATUS_FMT, "Site", name, fmt.Sprintf("↑: %s", util.BytesSizeAround(float64(ss.UserUploaded))),
		fmt.Sprintf("↓: %s", util.BytesSizeAround(float64(ss.UserDownloaded))), additionalInfo)
}

func PrintDummyStatus(f io.Writer, name string, info string) {
	if info != "" {
		info = "// " + info
	} else {
		info = "-"
	}
	fmt.Printf(constants.STATUS_FMT, "Site", name, "-", "-", info)
}

// Get real (number) id, removing sitename prefix
func (torrent *Torrent) ID() string {
	sitename, id, found := strings.Cut(torrent.Id, ".")
	if found {
		return id
	} else {
		return sitename
	}
}

func (torrent *Torrent) HasTag(tag string) bool {
	return slices.ContainsFunc(torrent.Tags, func(t string) bool {
		return strings.EqualFold(tag, t)
	})
}

// Return true if torrent has any tag of the tags list.
func (torrent *Torrent) HasAnyTag(tags []string) bool {
	return slices.ContainsFunc(tags, func(tag string) bool {
		return torrent.HasTag(tag)
	})
}

func (torrent *Torrent) MatchFilter(filter string) bool {
	if filter == "" || util.ContainsI(torrent.Name, filter) || util.ContainsI(torrent.Description, filter) {
		return true
	}
	return false
}

// Check if (seems) as a valid site status
func (status *Status) IsOk() bool {
	return status.UserName != "" || status.UserDownloaded > 0 || status.UserUploaded > 0
}

// Matches if any filter in list matches
func (torrent *Torrent) MatchFiltersOr(filters []string) bool {
	return slices.ContainsFunc(filters, func(filter string) bool {
		return torrent.MatchFilter(filter)
	})
}

// Matches if every list of filtersArray is successed with MatchFiltersOr().
// If filtersArray is empty, return true.
func (torrent *Torrent) MatchFiltersAndOr(filtersArray [][]string) bool {
	matched := true
	for _, includes := range filtersArray {
		if !torrent.MatchFiltersOr(includes) {
			matched = false
			break
		}
	}
	return matched
}

func Register(regInfo *RegInfo) {
	registryMap[regInfo.Name] = regInfo
	for _, alias := range regInfo.Aliases {
		registryMap[alias] = regInfo
	}
}

func CreateSiteInternal(name string,
	siteConfig *config.SiteConfigStruct, config *config.ConfigStruct) (Site, error) {
	regInfo := registryMap[siteConfig.Type]
	if regInfo == nil {
		return nil, fmt.Errorf("unsupported site type %s", name)
	}
	return regInfo.Creator(name, siteConfig, config)
}

func GetConfigSiteReginfo(name string) *RegInfo {
	for _, siteConfig := range config.Get().SitesEnabled {
		if siteConfig.GetName() == name {
			return registryMap[siteConfig.Type]
		}
	}
	return nil
}

func SiteExists(name string) bool {
	siteConfig := config.GetSiteConfig(name)
	return siteConfig != nil
}

func CreateSite(name string) (Site, error) {
	if sites[name] != nil {
		return sites[name], nil
	}
	siteConfig := config.GetSiteConfig(name)
	if siteConfig == nil {
		return nil, fmt.Errorf("site %s not found", name)
	}
	siteInstance, err := CreateSiteInternal(name, siteConfig, config.Get())
	if err != nil {
		sites[name] = siteInstance
	}
	return siteInstance, err
}

func PrintTorrents(output io.Writer, torrents []*Torrent, filter string, now int64,
	noHeader bool, dense bool, scores map[string]float64) {
	width, _, _ := term.GetSize(int(os.Stdout.Fd()))
	if width < config.SITE_TORRENTS_WIDTH {
		width = config.SITE_TORRENTS_WIDTH
	}
	widthExcludingName := 0
	widthName := 0
	if scores == nil {
		widthExcludingName = 89 // 6+11+19+4+4+4+23+2+8*2
		widthName = width - widthExcludingName
		if !noHeader {
			fmt.Fprintf(output, "%-*s  %6s  %-11s  %-19s  %4s  %4s  %4s  %-23s  %2s\n",
				widthName, "Name", "Size", "Free", "Time", "↑S", "↓L", "✓C", "ID", "P")
		}
	} else {
		widthExcludingName = 96 // 6+11+19+4+4+4+23+5+2+9*2
		widthName = width - widthExcludingName
		if !noHeader {
			fmt.Fprintf(output, "%-*s  %6s  %-11s  %-19s  %4s  %4s  %4s  %-23s  %5s  %2s\n",
				widthName, "Name", "Size", "Free", "Time", "↑S", "↓L", "✓C", "ID", "Score", "P")
		}
	}
	for _, torrent := range torrents {
		if filter != "" && !torrent.MatchFilter(filter) {
			continue
		}
		freeStr := ""
		if torrent.HasHnR {
			freeStr += "!"
		}
		if torrent.Paid {
			freeStr += "$"
		}
		if torrent.DownloadMultiplier == 0 {
			freeStr += "✓"
		} else {
			freeStr += "✕"
		}
		if torrent.DiscountEndTime > 0 {
			freeStr += fmt.Sprintf("(%s)", util.FormatDuration(torrent.DiscountEndTime-now))
		}
		if torrent.UploadMultiplier > 1 {
			freeStr = fmt.Sprintf("%1.1f", torrent.UploadMultiplier) + freeStr
		}
		if torrent.Neutral {
			freeStr += "N"
		} else if torrent.DownloadMultiplier == 0 && torrent.UploadMultiplier == 0 {
			freeStr += "Z"
		}
		name := torrent.Name
		process := "-"
		if torrent.IsActive {
			if torrent.IsCurrentActive {
				process = "*%"
			} else {
				process = "✓"
			}
		}
		if dense && (torrent.Description != "" || len(torrent.Tags) > 0) {
			name += " //"
			if torrent.Description != "" {
				name += " " + torrent.Description
			}
			if len(torrent.Tags) > 0 {
				name += fmt.Sprintf(" [%s]", strings.Join(util.Map(torrent.Tags, strconv.Quote), ", "))
			}
		}
		remain := util.PrintStringInWidth(output, name, int64(widthName), true)
		if scores == nil {
			fmt.Fprintf(output, "  %6s  %-11s  %-19s  %4d  %4d  %4d  %-23s  %2s\n",
				util.BytesSizeAround(float64(torrent.Size)),
				freeStr,
				util.FormatTime(torrent.Time),
				torrent.Seeders,
				torrent.Leechers,
				torrent.Snatched,
				torrent.Id,
				process,
			)
		} else {
			fmt.Fprintf(output, "  %6s  %-11s  %-19s  %4s  %4s  %4s  %-23s  %5.0f  %2s\n",
				util.BytesSizeAround(float64(torrent.Size)),
				freeStr,
				util.FormatTime(torrent.Time),
				fmt.Sprint(torrent.Seeders),
				fmt.Sprint(torrent.Leechers),
				fmt.Sprint(torrent.Snatched),
				torrent.Id,
				scores[torrent.Id],
				process,
			)
		}
		if dense {
			for {
				remain = strings.TrimSpace(remain)
				if remain == "" {
					break
				}
				remain = util.PrintStringInWidth(output, remain, int64(widthName), true)
				fmt.Fprintf(output, "\n")
			}
		}
	}
}

func GetConfigSiteNameByDomain(domain string) (string, error) {
	var firstMatchSite, lastMatchSite *config.SiteConfigStruct
	for _, siteConfig := range config.Get().SitesEnabled {
		if config.MatchSite(domain, siteConfig) {
			firstMatchSite = siteConfig
			break
		}
	}
	for i := len(config.Get().SitesEnabled) - 1; i >= 0; i-- {
		if config.MatchSite(domain, config.Get().SitesEnabled[i]) {
			lastMatchSite = config.Get().SitesEnabled[i]
			break
		}
	}
	if firstMatchSite != nil {
		if firstMatchSite == lastMatchSite {
			return firstMatchSite.GetName(), nil
		} else {
			return "",
				fmt.Errorf("ambiguous result - multiple sites in config match: names of first and last match: %s, %s",
					firstMatchSite.GetName(), lastMatchSite.GetName())
		}
	}
	return "", nil
}

func GetConfigSiteNameByTypes(types ...string) (string, error) {
	var firstMatchSite, lastMatchSite *config.SiteConfigStruct
	for _, siteConfig := range config.Get().SitesEnabled {
		if slices.Index(types, siteConfig.Type) != -1 {
			firstMatchSite = siteConfig
			break
		}
	}
	for i := len(config.Get().SitesEnabled) - 1; i >= 0; i-- {
		if slices.Index(types, config.Get().SitesEnabled[i].Type) != -1 {
			lastMatchSite = config.Get().SitesEnabled[i]
			break
		}
	}
	if firstMatchSite != nil {
		if firstMatchSite == lastMatchSite {
			return firstMatchSite.GetName(), nil
		} else {
			return "",
				fmt.Errorf("ambiguous result - multiple sites in config match: names of first and last match: %s, %s",
					firstMatchSite.GetName(), lastMatchSite.GetName())
		}
	}
	return "", nil
}

func CreateSiteHttpClient(siteConfig *config.SiteConfigStruct, globalConfig *config.ConfigStruct) (
	*azuretls.Session, [][]string, error) {
	var impersonate string
	var impersonateProfile *impersonateutil.Profile
	var httpHeaders [][]string
	if siteConfig.Impersonate != "" {
		impersonate = siteConfig.Impersonate
	} else if globalConfig.SiteImpersonate != "" {
		impersonate = globalConfig.SiteImpersonate
	}
	if impersonate != constants.NONE {
		if ip := impersonateutil.GetProfile(impersonate); ip == nil {
			return nil, nil, fmt.Errorf("impersonate '%s' not supported", impersonate)
		} else {
			impersonateProfile = ip
		}
	}
	ja3 := ""
	if siteConfig.Ja3 != "" {
		ja3 = siteConfig.Ja3
	} else if globalConfig.SiteJa3 != "" {
		ja3 = globalConfig.SiteJa3
	} else if impersonateProfile != nil {
		ja3 = impersonateProfile.Ja3
	}
	h2fingerprint := ""
	if siteConfig.H2Fingerprint != "" {
		h2fingerprint = siteConfig.H2Fingerprint
	} else if globalConfig.SiteH2Fingerprint != "" {
		h2fingerprint = globalConfig.SiteH2Fingerprint
	} else if impersonateProfile != nil {
		h2fingerprint = impersonateProfile.H2fingerprint
	}
	if impersonateProfile != nil && impersonateProfile.Headers != nil {
		httpHeaders = append(httpHeaders, impersonateProfile.Headers...)
	}
	if globalConfig.SiteHttpHeaders != nil {
		httpHeaders = append(httpHeaders, globalConfig.SiteHttpHeaders...)
	}
	if siteConfig.HttpHeaders != nil {
		httpHeaders = append(httpHeaders, siteConfig.HttpHeaders...)
	}
	proxy := config.GetProxy(siteConfig.Proxy, globalConfig.SiteProxy)
	if proxy == "" || proxy == constants.ENV_PROXY {
		proxy = util.ParseProxyFromEnv(siteConfig.Url)
	}
	insecure := config.Insecure || !siteConfig.Secure && (siteConfig.Insecure || globalConfig.SiteInsecure)
	timeout := util.FirstNonZeroIntegerArg(config.Timeout, siteConfig.Timeout, globalConfig.SiteTimeout)
	if timeout == 0 {
		timeout = config.DEFAULT_SITE_TIMEOUT
	} else if timeout < 0 {
		timeout = constants.INFINITE_TIMEOUT
	}
	if ja3 == constants.NONE {
		ja3 = ""
	}
	if h2fingerprint == constants.NONE {
		h2fingerprint = ""
	}
	if proxy == constants.NONE {
		proxy = ""
	}
	sep := "\n"
	specs := fmt.Sprint(ja3, sep, h2fingerprint, sep, proxy, sep, insecure, sep, timeout)
	log.Tracef("Create site %s http client with specs %s", siteConfig.GetName(), specs)
	hash := crypto.Md5String(specs)
	mu.Lock()
	defer mu.Unlock()
	if siteSessions[hash] != nil {
		return siteSessions[hash], httpHeaders, nil
	}
	session := azuretls.NewSession()
	session.SetTimeout(time.Duration(timeout) * time.Second)
	navigator := azuretls.Chrome
	if impersonateProfile != nil {
		navigator = impersonateProfile.Navigator
	}
	if ja3 != "" {
		if err := session.ApplyJa3(ja3, navigator); err != nil {
			return nil, nil, fmt.Errorf("failed to set ja3: %w", err)
		}
	}
	if h2fingerprint != "" {
		if err := session.ApplyHTTP2(h2fingerprint); err != nil {
			return nil, nil, fmt.Errorf("failed to set h2 finterprint: %w", err)
		}
	}
	if proxy != "" {
		if err := session.SetProxy(proxy); err != nil {
			return nil, nil, fmt.Errorf("failed to set proxy: %w", err)
		}
	}
	if insecure {
		session.InsecureSkipVerify = true
	}
	maxRedirects := config.DEFAULT_SITE_MAX_REDIRECTS
	if siteConfig.MaxRedirects != 0 {
		maxRedirects = siteConfig.MaxRedirects
	}
	session.MaxRedirects = uint(maxRedirects)
	siteSessions[hash] = session
	return session, httpHeaders, nil
}

// return site ua from siteConfig and globalConfig
func GetUa(siteInstance Site) string {
	ua := siteInstance.GetSiteConfig().UserAgent
	if ua == "" {
		ua = config.Get().SiteUserAgent
	}
	return ua
}

// General download torrent func. Return torrentContent, filename, err
func DownloadTorrentByUrl(siteInstance Site, httpClient *azuretls.Session, torrentUrl string, torrentId string) (
	[]byte, string, error) {
	res, header, err := util.FetchUrlWithAzuretls(torrentUrl, httpClient,
		siteInstance.GetSiteConfig().Cookie, GetUa(siteInstance), siteInstance.GetDefaultHttpHeaders())
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch torrents from site: %w", err)
	}
	mimeType, _, _ := mime.ParseMediaType(header.Get("content-type"))
	if mimeType != "" && mimeType != "application/octet-stream" && mimeType != "application/x-bittorrent" {
		return nil, "", fmt.Errorf("server return invalid content-type: %s", mimeType)
	}
	filename := util.ExtractFilenameFromHttpHeader(header)
	filenamePrefix := siteInstance.GetName()
	if torrentId != "" {
		filenamePrefix += "." + torrentId
	}
	if filename != "" {
		filename = fmt.Sprintf("%s.%s", filenamePrefix, filename)
	} else {
		filename = fmt.Sprintf("%s.torrent", filenamePrefix)
	}
	return res.Body, filename, err
}

// Do a multipart/form type post request to upload torrent to site, return site response.
// metadata is used as context when rendering payloadTemplate, the rendered payload be posted to site.
// All values in payload will be TrimSpaced.
// Some metadata names are specially handled:
//
//	_cover : cover image file path, got uploaded to site image server then replaced with uploaded img url.
//	_images (array) : images other than cover, processed similar with _cover but rendered as slice.
//	_raw_* : direct raw data that will be rendered and added to payload.
//	_site_<sitename>_* : site specific raw data.
//	_array_keys : variable of these keys are rendered as array.
func UploadTorrent(siteInstance Site, httpClient *azuretls.Session, uploadUrl string, contents []byte,
	metadata url.Values, fallbackPayloadTemplate map[string]string) (res *azuretls.Response, err error) {
	metadataRaw := map[string]any{}
	for key := range metadata {
		if slices.Contains(metadata[constants.METADATA_KEY_ARRAY_KEYS], key) {
			metadataRaw[key] = metadata[key]
		} else {
			metadataRaw[key] = metadata.Get(key)
		}
	}

	payload := url.Values{}
	payloadTemplate := siteInstance.GetSiteConfig().UploadTorrentPayload
	if payloadTemplate == nil {
		payloadTemplate = fallbackPayloadTemplate
	}
	if siteInstance.GetSiteConfig().UploadTorrentAdditionalPayload != nil {
		payloadTemplate = util.AssignMap(nil, payloadTemplate,
			siteInstance.GetSiteConfig().UploadTorrentAdditionalPayload)
	}

	coverFile := metadata.Get(constants.METADATA_KEY_COVER)
	imgPlaceholderPrefix := "$$PTOOL_PUBLISH_IMG_"
	// rendering occurs before images got uploaded, so use placeholders for now,
	// then replace them with uploaded image url in the later.
	// The con is image urls must be rendered literally inside payload templates.
	if coverFile != "" {
		metadataRaw[constants.METADATA_KEY_COVER] = imgPlaceholderPrefix + coverFile
	}
	if metadata.Has(constants.METADATA_KEY_IMAGES) {
		var images []string
		for _, imageFile := range metadata[constants.METADATA_KEY_IMAGES] {
			if imageFile == coverFile {
				continue
			}
			images = append(images, imgPlaceholderPrefix+imageFile)
		}
		metadataRaw[constants.METADATA_KEY_IMAGES] = images
	}
	for key := range payloadTemplate {
		value, err := jinja.Render(payloadTemplate[key], metadataRaw)
		if err != nil {
			return nil, fmt.Errorf("failed to render %s: %w", key, err)
		}
		payload.Set(key, value)
	}
	// raw payload directly set from metadata. e.g. "_raw_foo".
	for key := range metadata {
		if !strings.HasPrefix(key, "_raw_") {
			continue
		}
		template := metadata.Get(key)
		key = key[len("_raw_"):]
		if key == "" {
			continue
		}
		value, err := jinja.Render(template, metadataRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid metadata raw param %q: %w", key, err)
		}
		payload.Set(key, value)
	}
	// site specific raw payload directly set from metadata. e.g. "_site_kamept_foo".
	for key := range metadata {
		prefix := "_site_" + siteInstance.GetName() + "_"
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		template := metadata.Get(key)
		key = key[len(prefix):]
		if key == "" {
			continue
		}
		value, err := jinja.Render(template, metadataRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid metadata site raw param %q: %w", key, err)
		}
		payload.Set(key, value)
	}

	log.Debugf("Publish torrent payload: %v", payload)
	if keys := siteInstance.GetSiteConfig().UploadTorrentPayloadRequiredKeys; keys != "" && keys != constants.NONE {
		for _, key := range util.SplitCsv(keys) {
			if payload.Get(key) == "" {
				return nil, fmt.Errorf("required payload %s is not found or is empty", key)
			}
		}
	}

	// upload images.
	if coverFile != "" || metadata.Has(constants.METADATA_KEY_IMAGES) {
		if siteInstance.GetSiteConfig().ImageUploadUrl == "" {
			return nil, fmt.Errorf("imageUploadUrl is not configured, can not upload image")
		}
		var headers = [][]string{
			{"Referer", siteInstance.GetSiteConfig().ImageUploadUrl},
		}
		headers = append(headers, siteInstance.GetDefaultHttpHeaders()...)
		headers = append(headers, siteInstance.GetSiteConfig().ImageUploadHeaders...)
		headers = util.GetHttpReqHeaders(headers, "", "")
		var imageUploadPayload url.Values
		if siteInstance.GetSiteConfig().ImageUploadPayload != "" {
			imageUploadPayload, err = url.ParseQuery(siteInstance.GetSiteConfig().ImageUploadPayload)
			if err != nil {
				return nil, fmt.Errorf("invalid imageUploadPayload: %w", err)
			}
		}
		var images []string
		if coverFile != "" {
			images = append(images, coverFile)
		}
		images = append(images, metadata[constants.METADATA_KEY_IMAGES]...)
		for _, image := range images {
			if metadata.Has(constants.METADATA_KEY_DRY_RUN) {
				return nil, constants.ErrDryRun
			}
			imageUrl, err := util.PostUploadFileForUrl(httpClient, siteInstance.GetSiteConfig().ImageUploadUrl, image,
				nil, siteInstance.GetSiteConfig().ImageUploadFileField, imageUploadPayload, headers,
				siteInstance.GetSiteConfig().ImageUploadResponseUrlField)
			if err != nil {
				return nil, fmt.Errorf("failed to upload image %q: %w", image, err)
			}
			log.Debugf("uploaded image %q: url=%s", image, imageUrl)
			for key := range payload {
				value := payload.Get(key)
				newvalue := strings.ReplaceAll(value, imgPlaceholderPrefix+image, imageUrl)
				if newvalue != value {
					payload.Set(key, newvalue)
				}
			}
		}
	}
	if metadata.Has(constants.METADATA_KEY_DRY_RUN) {
		return nil, constants.ErrDryRun
	}
	headers := util.GetHttpReqHeaders(siteInstance.GetDefaultHttpHeaders(), siteInstance.GetSiteConfig().Cookie, "")
	if siteInstance.GetSiteConfig().Type == "nexusphp" {
		headers = append(headers, []string{"Referer", siteInstance.GetSiteConfig().ParseSiteUrl("upload.php", false)})
	}
	return util.PostUploadFile(httpClient, uploadUrl, "a.torrent", bytes.NewReader(contents), "file",
		payload, headers)
}

func init() {
}

// called by main codes on program exit. clean resources
func Exit() {
	var resourcesWaitGroup sync.WaitGroup
	// for now, nothing to close
	resourcesWaitGroup.Wait()
}

// Purge site cache
func Purge(sitename string) {
	if sitename == "" {
		for _, siteInstance := range sites {
			siteInstance.PurgeCache()
		}
	} else if sites[sitename] != nil {
		sites[sitename].PurgeCache()
	}
}
