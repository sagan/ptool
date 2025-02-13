package util

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// offset: if > 0, indicates the bytes offset of the end of found time string in original str
func ExtractTime(str string, location *time.Location) (time int64, offset int64) {
	timeRegexp := regexp.MustCompile(`.*?(?P<time>\d{4}-\d{2}-\d{2}\s*\d{2}:\d{2}:\d{2})`)
	m := timeRegexp.FindStringSubmatch(str)
	if m != nil {
		str = m[timeRegexp.SubexpIndex("time")]
		offset = int64(len(m[0]))
	}
	time, err := ParseTime(str, location)
	if err != nil {
		offset = 0
	}
	return
}

func FormatDate(ts int64) string {
	return time.Unix(ts, 0).Format("2006-01-02")
}

func FormatDate2(ts int64) string {
	return time.Unix(ts, 0).Format("20060102")
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

func FormatTime(ts int64) string {
	return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
}

func Now() int64 {
	return time.Now().Unix()
}

func ParseFutureTime(str string) (int64, error) {
	if td, err := ParseTimeDuration(str); err == nil {
		return time.Now().Unix() + td, nil
	}
	return 0, fmt.Errorf("invalid time str")
}

func ParseLocalDateTime(str string) (int64, error) {
	if t, err := time.ParseInLocation("2006-01-02", str, time.Local); err == nil {
		return t.Unix(), nil
	}
	return 0, fmt.Errorf("invalid date str")
}

// Similar with ParseTimeWithNow but use time.Now() as now time
func ParseTime(str string, location *time.Location) (int64, error) {
	return ParseTimeWithNow(str, location, time.Now())
}

// Parse time (with date) string. .
// It try to parse str in any of the below time format:
// "yyyy-MM-ddHH:mm:ss", "yyyy-MM-dd HH:mm:ss", "yyyy-MM-ddTHH:mm:ssZ", <integer> (unix timestamp in seconds),
// "time duration" (e.g. "5d", "6hm5s", "4天5时") (treat as pasted time til now)
// If location is nil, current local timezone is used.
func ParseTimeWithNow(str string, location *time.Location, now time.Time) (int64, error) {
	str = strings.TrimSpace(str)
	if str == "" {
		return 0, fmt.Errorf("empty str")
	}
	if i, err := strconv.Atoi(str); err == nil {
		return int64(i), nil
	}
	if strings.Contains(str, ",") {
		if i, err := strconv.Atoi(strings.ReplaceAll(str, ",", "")); err == nil {
			return int64(i), nil
		}
	}

	if t, err := time.Parse("2006-01-02T15:04:05Z", str); err == nil {
		return t.Unix(), nil
	}
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-0215:04:05",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02",
	}
	if location == nil {
		location = time.Local
	}
	for _, format := range formats {
		if t, err := time.ParseInLocation(format, str, location); err == nil {
			return t.Unix(), nil
		}
	}

	if td, err := ParseTimeDuration(str); err == nil {
		t := time.Unix(now.Unix()-td, 0)
		// 以距今的相对时间标识，精度有限
		if td%86400 == 0 && td >= 86400*30 { // e.g. "1月0天", "1年10月"
			t = t.Truncate(time.Hour * 24) // Go standard library truncates time against UTC
			t = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, location)
		} else if td%3600 == 0 && td >= 86400 { // e.g. "1天0时", "29天10时"
			t = t.Truncate(time.Hour)
		} else if td%60 == 0 && td >= 3600 { // e.g. "1时25分钟"
			t = t.Truncate(time.Minute)
		}
		return t.Unix(), nil
	}
	return 0, fmt.Errorf("invalid time str")
}

// Return time duration in seconds
func ParseTimeDuration(str string) (int64, error) {
	// remove inner spaces like the one in "9 小时"
	var re = regexp.MustCompile(`^(.*?)\s*(\D+)\s*(.*?)$`)
	for {
		str1 := re.ReplaceAllString(str, `$1$2$3`)
		if str1 == str {
			break
		}
		str = str1
	}
	str = strings.ReplaceAll(str, "年", "y")
	str = strings.ReplaceAll(str, "月", "M")
	str = strings.ReplaceAll(str, "周", "w")
	str = strings.ReplaceAll(str, "天", "d")
	str = strings.ReplaceAll(str, "日", "d")
	str = strings.ReplaceAll(str, "小时", "h")
	str = strings.ReplaceAll(str, "小時", "h")
	str = strings.ReplaceAll(str, "时", "h")
	str = strings.ReplaceAll(str, "時", "h")
	str = strings.ReplaceAll(str, "分钟", "m")
	str = strings.ReplaceAll(str, "分鐘", "m")
	str = strings.ReplaceAll(str, "分", "m")
	str = strings.ReplaceAll(str, "秒", "s")
	str = strings.TrimSuffix(str, "前")
	td, err := ParseDuration(str)
	if err == nil {
		return int64(td.Seconds()), nil
	}
	return 0, err
}
