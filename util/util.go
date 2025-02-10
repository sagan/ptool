package util

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"time"

	"github.com/KarpelesLab/reflink"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/constraints"
)

var commaSeperatorRegexp = regexp.MustCompile(`\s*,\s*`)

// split a csv like line to values. "a, b, c" => [a,b,c].
// If str is empty string, return nil.
func SplitCsv(str string) []string {
	if str == "" {
		return nil
	}
	return commaSeperatorRegexp.Split(str, -1)
}

func String2Any(value string) (any, reflect.Kind) {
	if value == "true" {
		return true, reflect.Bool
	} else if value == "false" {
		return false, reflect.Bool
	} else if IsIntString(value) {
		return ParseInt(value), reflect.Int64
	} else {
		return value, reflect.String
	}
}

func ResolvePointerValue(obj any) any {
	ref := reflect.ValueOf(obj)
	if ref.Kind() == reflect.Ptr {
		obj = reflect.Indirect(ref).Interface()
	}
	return obj
}

func GetStructFieldValue(obj any, field string, defaultValue any) any {
	ref := reflect.ValueOf(obj)

	if ref.Kind() == reflect.Ptr {
		ref = reflect.Indirect(ref)
	}
	prop := ref.FieldByName(field)
	if !prop.IsValid() {
		return defaultValue
	}
	return prop.Interface()
}

// https://stackoverflow.com/questions/6395076/using-reflect-how-do-you-set-the-value-of-a-struct-field
func SetStructFieldValue(obj any, field string, value any) {
	ref := reflect.ValueOf(obj)

	if ref.Kind() != reflect.Ptr {
		log.Fatalf("SetStructFieldValue: you must pass obj as a pointer")
	}
	ref = reflect.Indirect(ref)

	if ref.Kind() == reflect.Interface {
		ref = ref.Elem()
	}

	// should double check we now have a struct (could still be anything)
	if ref.Kind() != reflect.Struct {
		log.Fatalf("SetStructFieldValue field %s: unexpected type", field)
	}

	prop := ref.FieldByName(field)
	prop.Set(reflect.ValueOf(value))
}

// https://stackoverflow.com/questions/23350173
// copy non-empty field values from src to dst. dst and src must be pointors of same type of plain struct
func Assign(dst any, src any, excludeFieldIndexes []int) {
	dstValue := reflect.ValueOf(dst).Elem()
	srcValue := reflect.ValueOf(src).Elem()

	for i := 0; i < dstValue.NumField(); i++ {
		dstField := dstValue.Field(i)
		srcField := srcValue.Field(i)
		fieldType := dstField.Type()
		srcValue := reflect.Value(srcField)
		if slices.Contains(excludeFieldIndexes, i) {
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
		if (fieldType.Kind() == reflect.Slice || fieldType.Kind() == reflect.Map ||
			fieldType.Kind() == reflect.Pointer) && srcValue.Pointer() == 0 {
			continue
		}
		dstField.Set(srcValue)
	}
}

// similar to JavaScript's Object.assign(args[0], args[1], args[2]...), update and return args[0].
// However, if args[0] is nil, create and return a new map instead; if any other arg is nil, ignore it
func AssignMap[T1 comparable, T2 any](args ...map[T1]T2) map[T1]T2 {
	if len(args) == 0 {
		return nil
	}
	result := args[0]
	for i := 1; i < len(args); i++ {
		if result == nil && len(args[i]) > 0 {
			result = map[T1]T2{}
		}
		for key, value := range args[i] {
			result[key] = value
		}
	}
	return result
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

// return a non-existing filename
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

func First[T1 any, T2 any](v T1, args ...T2) T1 {
	return v
}

// Parse standard HTTP_PROXY, HTTPS_PROXY, NO_PROXY (and lowercase versions) envs, return proxy for urlStr.
func ParseProxyFromEnv(urlStr string) string {
	if urlStr == "" {
		return ""
	}
	urlObj, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	proxyUrl, err := http.ProxyFromEnvironment(&http.Request{URL: urlObj})
	if err != nil || proxyUrl == nil {
		return ""
	}
	return proxyUrl.String()
}

// From https://stackoverflow.com/questions/21060945/simple-way-to-copy-a-file .
// Copy copies the contents of the file at srcpath to a regular file
// at dstpath. If the file named by dstpath already exists, it is
// truncated. The function does not copy the file mode, file
// permission bits, or file attributes.
func CopyFile(srcpath, dstpath string) (err error) {
	r, err := os.Open(srcpath)
	if err != nil {
		return err
	}
	defer r.Close() // ignore error: file was opened read-only.

	w, err := os.Create(dstpath)
	if err != nil {
		return err
	}

	defer func() {
		// Report the error, if any, from Close, but do so
		// only if there isn't already an outgoing error.
		if c := w.Close(); err == nil {
			err = c
		}
	}()

	_, err = io.Copy(w, r)
	return err
}

// Create hardlink duplicate for source dir at dest. Recursively process all files and folders inside source.
// Symbolinks are ignored.
// For file with size < limit, create a copy instead.
func LinkDir(source string, dest string, limit int64, useReflink bool) error {
	return filepath.WalkDir(source, func(sourcePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relativePath, err := filepath.Rel(source, sourcePath)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dest, relativePath)
		if d.IsDir() {
			log.Tracef("Create dir %s", destPath)
			err := os.Mkdir(destPath, d.Type())
			if err != nil && !os.IsExist(err) {
				return err
			}
		} else if d.Type().IsRegular() {
			if stat, err := os.Stat(sourcePath); err != nil {
				return err
			} else if useReflink {
				log.Tracef("Reflink %s => %s", sourcePath, destPath)
				// Don't use reflink.Auto because reflink fallbacks to copying, not hard linking
				if err := reflink.Always(sourcePath, destPath); err != nil {
					return err
				}
			} else if limit >= 0 && stat.Size() < limit {
				log.Tracef("Copy %s => %s", sourcePath, destPath)
				if err := CopyFile(sourcePath, destPath); err != nil {
					return err
				}
			} else {
				log.Tracef("Link %s => %s", sourcePath, destPath)
				if err := os.Link(sourcePath, destPath); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// Check whether a file (or dir) with name exists in file system.
// It treat a file system error as file exits.
func FileExists(name string) bool {
	if _, err := os.Stat(name); err == nil || !os.IsNotExist(err) {
		return true
	}
	return false
}

// Return true if name is a accessible dir.
func DirExists(name string) bool {
	if stat, err := os.Stat(name); err == nil && stat.IsDir() {
		return true
	}
	return false
}

func TouchFile(name string) error {
	file, err := os.OpenFile(name, os.O_RDONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	now := time.Now()
	os.Chtimes(name, now, now)
	return file.Close()
}

// Check whether a file (or dir) with name or name + suffix exists in file system.
// suffix could be any one in suffixes.
func FileExistsWithOptionalSuffix(name string, suffixes ...string) bool {
	if FileExists(name) {
		return true
	}
	for _, suffix := range suffixes {
		if FileExists(name + suffix) {
			return true
		}
	}
	return false
}

func ExistsFileWithAnySuffix(name string, suffixes []string) string {
	for _, suffix := range suffixes {
		if path := name + suffix; FileExists(path) {
			return path
		}
	}
	return ""
}

// Return count of variable in vars that fulfil the condition that variable is non-zero value
func CountNonZeroVariables(vars ...any) (cnt int) {
	for _, variable := range vars {
		switch v := variable.(type) {
		case string:
			if v != "" {
				cnt++
			}
		case int:
			if v != 0 {
				cnt++
			}
		case int64:
			if v != 0 {
				cnt++
			}
		case float64:
			if v != 0 {
				cnt++
			}
		case bool:
			if v {
				cnt++
			}
		case []string:
			if len(v) > 0 {
				cnt++
			}
		default:
			panic("unsupported type")
		}
	}
	return
}

func FirstNonZeroIntegerArg[T constraints.Integer](args ...T) T {
	for _, t := range args {
		if t != 0 {
			return t
		}
	}
	return 0
}

func BytesHasAnyStringPrefix(data []byte, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if bytes.HasPrefix(data, []byte(prefix)) {
			return true
		}
	}
	return false
}

// Print value json string to output.
// It prints a trailing \n
func PrintJson(output io.Writer, value any) error {
	bytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}
	fmt.Fprintln(output, string(bytes))
	return nil
}
