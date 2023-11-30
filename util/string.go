package util

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/jpillora/go-tld"
	runewidth "github.com/mattn/go-runewidth"
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

func IsUrl(str string) bool {
	return strings.HasPrefix(str, "http://") || strings.HasPrefix(str, "https://")
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

// return prefix of string at most width and actual width.
// ASCII char has 1 width. CJK char has 2 width
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

func PrintStringInWidth(str string, width int64, padRight bool) {
	pstr, strWidth := StringPrefixInWidth(str, width)
	if padRight {
		pstr += strings.Repeat(" ", int(width-strWidth))
	} else {
		pstr = strings.Repeat(" ", int(width-strWidth)) + pstr
	}
	fmt.Print(pstr)
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

// return (top-level) domain of a url. eg. https://www.google.com/ => google.com
func GetUrlDomain(url string) string {
	u, err := tld.Parse(url)
	if err != nil {
		return ""
	}
	return u.Domain + "." + u.TLD
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

// Splitting a string at Space, except inside quotation marks.
// from https://stackoverflow.com/questions/47489745/splitting-a-string-at-space-except-inside-quotation-marks
func ParseArgs(s string) []string {
	a := []string{}
	sb := &strings.Builder{}
	quoted := false
	for _, r := range s {
		if r == '"' {
			if quoted {
				a = append(a, sb.String())
				sb.Reset()
			}
			quoted = !quoted
		} else if !quoted {
			if unicode.IsSpace(r) {
				if sb.Len() > 0 {
					a = append(a, sb.String())
					sb.Reset()
				}
			} else {
				sb.WriteRune(r)
			}
		} else {
			sb.WriteRune(r)
		}
	}
	if sb.Len() > 0 {
		a = append(a, sb.String())
	}
	return a
}
