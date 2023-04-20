package utils

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	runewidth "github.com/mattn/go-runewidth"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/constraints"
	"golang.org/x/exp/slices"
)

var (
	ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/111.0.0.0 Safari/537.36"
)

func ContainsI(str string, substr string) bool {
	return strings.Contains(
		strings.ToLower(str),
		strings.ToLower(substr),
	)
}

func SetHttpRequestBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", ua)
	req.Header.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("accept-language", "en")
	req.Header.Set("cache-control", "no-cache")
	req.Header.Set("pragma", "no-cache")
}

func FetchUrl(url string, cookie string, client *http.Client) (*http.Response, error) {
	log.Tracef("FetchUrl url=%s hasCookie=%t", url, cookie != "")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	SetHttpRequestBrowserHeaders(req)
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, error := client.Do(req)
	if error != nil {
		return nil, fmt.Errorf("failed to fetch url: %v", error)
	}
	log.Tracef("FetchUrl resp status=%d", resp.StatusCode)
	if resp.StatusCode != 200 {
		defer resp.Body.Close()
		return nil, fmt.Errorf("failed to fetch url: status=%d", resp.StatusCode)
	}
	return resp, nil
}

func GetUrlDoc(url string, cookie string, client *http.Client) (*goquery.Document, error) {
	res, err := FetchUrl(url, cookie, client)
	if err != nil {
		return nil, fmt.Errorf("can not fetch site data %v", err)
	}
	defer res.Body.Close()
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse site page DOM, error: %v", err)
	}
	return doc, nil
}

func ParseInt(str string) int64 {
	str = strings.ReplaceAll(str, ",", "")
	v, _ := strconv.ParseInt(str, 10, 0)
	return v
}

func FormatDuration(seconds int64) (str string) {
	dStr := ""
	hStr := ""
	mStr := ""
	sStr := ""

	d := seconds / 86400
	if d > 0 {
		dStr = fmt.Sprint(d, "d")
	}
	seconds %= 86400

	h := seconds / 3600
	if h > 0 {
		hStr = fmt.Sprint(h, "h")
	}
	seconds %= 3600

	m := seconds / 60
	if m > 0 {
		mStr = fmt.Sprint(m, "m")
	}
	seconds %= 60

	if seconds > 0 {
		sStr = fmt.Sprint(seconds, "s")
	}

	strs := []string{dStr, hStr, mStr, sStr}
	i := 0
	for _, s := range strs {
		if s != "" {
			str += s
			i++
		}
		if i == 2 {
			break
		}
	}
	return
}

func ParseTimeDuration(str string) (int64, error) {
	str = strings.ReplaceAll(str, "周", "w")
	str = strings.ReplaceAll(str, "天", "d")
	str = strings.ReplaceAll(str, "日", "d")
	str = strings.ReplaceAll(str, "小时", "h")
	str = strings.ReplaceAll(str, "小時", "h")
	str = strings.ReplaceAll(str, "时", "h")
	str = strings.ReplaceAll(str, "時", "h")
	str = strings.ReplaceAll(str, "分种", "m")
	str = strings.ReplaceAll(str, "分鐘", "m")
	str = strings.ReplaceAll(str, "分", "m")
	str = strings.ReplaceAll(str, "秒", "s")
	td, error := ParseDuration(str)
	if error == nil {
		return int64(td.Seconds()), nil
	}
	return 0, error
}

func ParseFutureTime(str string) (int64, error) {
	td, error := ParseTimeDuration(str)
	if error == nil {
		return time.Now().Unix() + td, nil
	}
	return 0, fmt.Errorf("invalid time str")
}

func ExtractTime(str string, location *time.Location) (time int64) {
	timeRegexp := regexp.MustCompile(`(?P<time>\d{4}-\d{2}-\d{2}\s*\d{2}:\d{2}:\d{2})`)
	m := timeRegexp.FindStringSubmatch(str)
	if m != nil {
		time, _ = ParseTime(m[timeRegexp.SubexpIndex("time")], location)
	}
	return
}

// parse time. Treat duration time as pasted
func ParseTime(str string, location *time.Location) (int64, error) {
	str = strings.TrimSpace(str)
	if str == "" {
		return 0, fmt.Errorf("empty str")
	}
	//  handle YYYY-mm-ddHH:mm:ss
	if matched, _ := regexp.MatchString("\\d{4}-\\d{2}-\\d{2}\\d{2}:\\d{2}:\\d{2}", str); matched {
		str = str[:10] + " " + str[10:]
	}

	if location == nil {
		location = time.Local
	}
	t, error := time.ParseInLocation("2006-01-02 15:04:05", str, location)
	if error == nil {
		return t.Unix(), nil
	}

	td, error := ParseTimeDuration(str)
	if error == nil {
		return time.Now().Unix() - td, nil
	}
	return 0, fmt.Errorf("invalid time str")
}

func ParseLocalDateTime(str string) (int64, error) {
	t, error := time.ParseInLocation("2006-01-02", str, time.Local)
	if error == nil {
		return t.Unix(), nil
	}
	return 0, fmt.Errorf("invalid date str")
}

func FormatDate(ts int64) string {
	return time.Unix(ts, 0).Format("2006-01-02")
}

func FormatDate2(ts int64) string {
	return time.Unix(ts, 0).Format("20060102")
}

func FormatTime(ts int64) string {
	return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
}

func Now() int64 {
	return time.Now().Unix()
}

func Filter[T any](ss []T, test func(T) bool) (ret []T) {
	for _, s := range ss {
		if test(s) {
			ret = append(ret, s)
		}
	}
	return
}

func MapMaxElementKey[TK comparable, TV constraints.Ordered](m map[TK](TV)) TK {
	var result TK
	var resultValue TV
	i := int64(0)
	for key, value := range m {
		if i == 0 {
			result = key
			resultValue = value
		} else if value > resultValue {
			result = key
			resultValue = value
		}
		i++
	}
	return result
}

func CopyMap[T1 comparable, T2 any](m map[T1](T2)) map[T1](T2) {
	cp := make(map[T1](T2))
	for k, v := range m {
		cp[k] = v
	}
	return cp
}

func FindInSlice[T any](slice []T, checker func(T) bool) *T {
	index := slices.IndexFunc(slice, checker)
	if index == -1 {
		return nil
	}
	return &slice[index]
}

// https://stackoverflow.com/questions/18537257/how-to-get-the-directory-of-the-currently-running-file
func SelfDir() string {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return filepath.Dir(ex)
}

func Sleep(seconds int64) {
	time.Sleep(time.Duration(seconds) * time.Second)
}

// https://stackoverflow.com/questions/23350173
// copy none-empty field values from src to dst. dst and src must be pointors of same type of plain struct
func Assign(dst any, src any) {
	dstValue := reflect.ValueOf(dst).Elem()
	srcValue := reflect.ValueOf(src).Elem()

	for i := 0; i < dstValue.NumField(); i++ {
		dstField := dstValue.Field(i)
		srcField := srcValue.Field(i)
		fieldType := dstField.Type()
		srcValue := reflect.Value(srcField)
		if fieldType.Kind() == reflect.String && srcValue.String() == "" {
			continue
		}
		if fieldType.Kind() == reflect.Int64 && srcValue.Int() == 0 {
			continue
		}
		if fieldType.Kind() == reflect.Float64 && srcValue.Float() == 0 {
			continue
		}
		if fieldType.Kind() == reflect.Bool && !srcValue.Bool() {
			continue
		}
		dstField.Set(srcValue)
	}
}

func ParseUrlHostname(urlStr string) string {
	hostname := ""
	url, err := url.Parse(urlStr)
	if err == nil {
		hostname = url.Hostname()
	}
	return hostname
}

func DomHtml(el *goquery.Selection) string {
	html, _ := el.Html()
	return html
}

func DomSanitizedText(el *goquery.Selection) string {
	text := el.Text()
	text = strings.ReplaceAll(text, "\u00ad", "") // &shy;  invisible Soft hyphen
	text = strings.TrimSpace(text)
	return text
}

func DomRemovedSpecialCharsText(node *goquery.Selection) string {
	str := DomSanitizedText(node)
	m := regexp.MustCompile(`[-\[\]\(\)【】（）：:]`)
	str = m.ReplaceAllString(str, " ")
	return str
}

func Capitalize(str string) string {
	return strings.ToUpper(str[:1]) + str[1:]
}

func PrintStringInWidth(str string, width int64, padRight bool) {
	strWidth := int64(0)
	pstr := ""
	for _, char := range str {
		runeWidth := int64(runewidth.RuneWidth(char))
		if strWidth+runeWidth > width {
			break
		}
		pstr += string(char)
		strWidth += runeWidth
	}
	if padRight {
		pstr += strings.Repeat(" ", int(width-strWidth))
	} else {
		pstr = strings.Repeat(" ", int(width-strWidth)) + pstr
	}
	fmt.Print(pstr)
}
