package utils

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"golang.org/x/exp/constraints"
	"golang.org/x/exp/slices"
)

// https://stackoverflow.com/questions/23350173
// copy none-empty field values from src to dst. dst and src must be pointors of same type of plain struct
func Assign(dst any, src any, excludeFieldIndexes []int) {
	dstValue := reflect.ValueOf(dst).Elem()
	srcValue := reflect.ValueOf(src).Elem()

	for i := 0; i < dstValue.NumField(); i++ {
		dstField := dstValue.Field(i)
		srcField := srcValue.Field(i)
		fieldType := dstField.Type()
		srcValue := reflect.Value(srcField)
		if slices.Index(excludeFieldIndexes, i) != -1 {
			continue
		}
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

func Max[T constraints.Ordered](args ...T) T {
	max := args[0]
	for _, x := range args {
		if x > max {
			max = x
		}
	}
	return max
}

func Min[T constraints.Ordered](args ...T) T {
	min := args[0]
	for _, x := range args {
		if x < min {
			min = x
		}
	}
	return min
}

// https://stackoverflow.com/questions/18537257/how-to-get-the-directory-of-the-currently-running-file
func SelfDir() string {
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return filepath.Dir(ex)
}

func Sha1(s []byte) string {
	h := sha1.New()
	h.Write(s)
	return hex.EncodeToString(h.Sum(nil))
}

func Sha1String(s string) string {
	return Sha1([]byte(s))
}

func Sleep(seconds int64) {
	time.Sleep(time.Duration(seconds) * time.Second)
}

// return a none-existing filename
func GetNewFilename(filename string) string {
	if _, err := os.Stat(filename); errors.Is(err, os.ErrNotExist) {
		return filename
	}
	id := int64(0)
	for {
		id++
		filenameWithId := fmt.Sprintf("%s (%d)", filename, id)
		if _, err := os.Stat(filenameWithId); errors.Is(err, os.ErrNotExist) {
			return filenameWithId
		}
	}
}

// "*.torrent" => ["a.torrent", "b.torrent"...].
// Windows cmd / powershell 均不支持命令行 *.torrent 参数扩展。必须应用自己实现。做个简易版的
func GetWildcardFilenames(filestr string) []string {
	dir := filepath.Dir(filestr)
	name := filepath.Base(filestr)
	if name == "" {
		return nil
	}
	ext := filepath.Ext(name)
	if ext != "" {
		name = name[:len(name)-len(ext)]
	}
	prefix := ""
	suffix := ""
	if !strings.Contains(name, "*") {
		return nil
	} else if name != "*" {
		if strings.HasPrefix(name, "*") {
			suffix = name[1:]
		} else if strings.HasSuffix(name, "*") {
			prefix = name[:len(name)-1]
		} else {
			return nil // not supported yet
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	filenames := []string{}
	for _, entry := range entries {
		entryExt := filepath.Ext(entry.Name())
		if entryExt != ext {
			continue
		}
		if prefix != "" {
			if strings.HasPrefix(entry.Name(), prefix) {
				filenames = append(filenames, dir+"/"+entry.Name())
			}
		} else if suffix != "" {
			if strings.HasSuffix(entry.Name(), suffix) {
				filenames = append(filenames, dir+"/"+entry.Name())
			}
		} else {
			filenames = append(filenames, dir+"/"+entry.Name())
		}
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
