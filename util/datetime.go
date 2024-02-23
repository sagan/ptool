package util

import (
	"fmt"
	"regexp"
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
	td, error := ParseTimeDuration(str)
	if error == nil {
		return time.Now().Unix() + td, nil
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

// Parse time. Treat duration time as pasted.
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
		now := time.Now()
		// 以距今的相对时间标识，精度有限
		if td%86400 == 0 {
			if td > 86400*30 {
				now = now.Truncate(time.Hour * 24)
			} else if td > 86400 {
				now = now.Truncate(time.Hour)
			}
		}
		return now.Unix() - td, nil
	}
	return 0, fmt.Errorf("invalid time str")
}

// Return time duration in seconds
func ParseTimeDuration(str string) (int64, error) {
	// remove inside spaces like the one in "9 小时"
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
	td, error := ParseDuration(str)
	if error == nil {
		return int64(td.Seconds()), nil
	}
	return 0, error
}
