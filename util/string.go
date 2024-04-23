package util

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jpillora/go-tld"
	runewidth "github.com/mattn/go-runewidth"
)

var (
	// It's far from strict for now
	hostnameRegex = regexp.MustCompile(`^([a-zA-Z0-9][-a-zA-Z0-9]*\.)+([a-zA-Z0-9][-a-zA-Z0-9]*)$`)
)

func Capitalize(str string) string {
	if len(str) == 0 {
		return str
	}
	return strings.ToUpper(str[:1]) + str[1:]
}

func ContainsI(str string, substr string) bool {
	return strings.Contains(
		strings.ToLower(str),
		strings.ToLower(substr),
	)
}

// Check whether str is a "http://" or "https://"" url
func IsUrl(str string) bool {
	return strings.HasPrefix(str, "http://") || strings.HasPrefix(str, "https://")
}

// Check whether str is a url of "magnet:" or "bc://bt/" schema.
func IsPureTorrentUrl(str string) bool {
	return strings.HasPrefix(str, "magnet:") || strings.HasPrefix(str, "bc://bt/")
}

// Check whether str is a normal (http / https schema) or torrent (magnet / bt schema) url
func IsTorrentUrl(str string) bool {
	return IsUrl(str) || IsPureTorrentUrl(str)
}

// Parse a baseUrl relative relativeUrl, return absolute url.
// baseUrl could also be a host, in which case https schema is assumed.
func ParseRelativeUrl(relativeUrl string, baseUrl string) string {
	if IsUrl(relativeUrl) || baseUrl == "" {
		return relativeUrl
	}
	if !IsUrl(baseUrl) {
		baseUrl = "https://" + baseUrl
	}
	return strings.TrimSuffix(baseUrl, "/") + "/" + strings.TrimPrefix(relativeUrl, "/")
}

func IsHostname(str string) bool {
	return hostnameRegex.MatchString(str)
}

func IsHexString(str string, minLength int) bool {
	reg := regexp.MustCompile(fmt.Sprintf(`^[a-fA-F0-9]{%d,}$`, minLength))
	return reg.MatchString(str)
}

func IsIntString(str string) bool {
	return regexp.MustCompile(`^\d+$`).MatchString(str)
}

func ParseInt(str string) int64 {
	str = strings.TrimSpace(strings.ReplaceAll(str, ",", ""))
	v, _ := strconv.ParseInt(str, 10, 0)
	return v
}

// Return prefix of str that is at most max bytes encoded in UTF-8
func StringPrefixInBytes(str string, max int64) string {
	length := 0
	sb := &strings.Builder{}
	for _, char := range str {
		runeLength := utf8.RuneLen(char)
		if length+runeLength > int(max) {
			break
		}
		sb.WriteRune(char)
		length += runeLength
	}
	return sb.String()
}

// Return prefix of string at most width and actual width.
// ASCII char has 1 width. CJK char has 2 width.
func StringPrefixInWidth(str string, width int64) (string, int64) {
	strWidth := int64(0)
	sb := &strings.Builder{}
	for _, char := range str {
		runeWidth := int64(runewidth.RuneWidth(char))
		if strWidth+runeWidth > width {
			break
		}
		sb.WriteRune(char)
		strWidth += runeWidth
	}
	return sb.String(), strWidth
}

func PrintStringInWidth(str string, width int64, padRight bool) (remain string) {
	pstr, strWidth := StringPrefixInWidth(str, width)
	remain = str[len(pstr):]
	if padRight {
		pstr += strings.Repeat(" ", int(width-strWidth))
	} else {
		pstr = strings.Repeat(" ", int(width-strWidth)) + pstr
	}
	fmt.Print(pstr)
	return
}

func SanitizeText(text string) string {
	text = strings.ReplaceAll(text, "\u00ad", "")  // &shy;  invisible Soft hyphen
	text = strings.ReplaceAll(text, "\u00a0", " ") // non-breaking space => normal space (U+0020)
	text = strings.TrimSpace(text)
	return text
}

// append a proper ? or & to url
func AppendUrlQueryStringDelimiter(url string) string {
	if !strings.HasSuffix(url, "?") {
		if !strings.Contains(url, "?") {
			url += "?"
		} else if !strings.HasSuffix(url, "&") {
			url += "&"
		}
	}
	return url
}

func AppendUrlQueryString(url string, qs string) string {
	url = AppendUrlQueryStringDelimiter(url)
	if strings.HasPrefix(qs, "?") || strings.HasPrefix(qs, "&") {
		qs = qs[1:]
	}
	return url + qs
}

var nonTldSiteDomains = map[string][]string{
	"eu.org": {"ecustpt.eu.org"},
	"pp.ua":  {"ecust.pp.ua"},
}

// Return (top-level) domain of a url. e.g. https://www.google.com/ => google.com.
// Very few PT sites do NOT use top-level domain, url of those sites are handled specially,
// with their (second level) site domain returned.
func GetUrlDomain(urlStr string) string {
	if urlStr == "" {
		return ""
	}
	u, err := tld.Parse(urlStr)
	if err != nil {
		return ""
	}
	domain := u.Domain + "." + u.TLD
	if nonTldSiteDomains[domain] != nil {
		if urlObj, err := url.Parse(urlStr); err == nil {
			for _, domain := range nonTldSiteDomains[domain] {
				if urlObj.Hostname() == domain || strings.HasSuffix(urlObj.Hostname(), "."+domain) {
					return domain
				}
			}
		}
	}
	return domain
}

var sizeStrRegex = regexp.MustCompile(`(?P<size>[0-9,]{1,}(.[0-9,]{1,})?\s*[kbgtpeKMGTPE][iI]?[bB]+)`)

func ExtractSizeStr(str string) (int64, error) {
	m := sizeStrRegex.FindStringSubmatch(str)
	if m != nil {
		sstr := strings.ReplaceAll(strings.TrimSpace(m[sizeStrRegex.SubexpIndex("size")]), " ", "")
		sstr = strings.ReplaceAll(sstr, ",", "")
		return RAMInBytes(sstr)
	}
	return 0, fmt.Errorf("no size str found")
}

func QuoteFilename(str string) string {
	hasSpacialChars := strings.ContainsAny(str, " '\r\n\t\b\"")
	if hasSpacialChars {
		str = strings.ReplaceAll(str, "\r", " ")
		str = strings.ReplaceAll(str, "\n", " ")
		str = strings.ReplaceAll(str, `"`, `\"`)
		str = `"` + str + `"`
	}
	return str
}

func HasAnySuffix(str string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(str, suffix) {
			return true
		}
	}
	return false
}

func TrimAnySuffix(str string, suffixes ...string) string {
	for _, suffix := range suffixes {
		if strings.HasSuffix(str, suffix) {
			return strings.TrimSuffix(str, suffix)
		}
	}
	return str
}
