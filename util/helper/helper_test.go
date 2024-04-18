package helper_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sagan/ptool/util/helper"
)

func TestGetWildcardFilenames(t *testing.T) {
	dir := t.TempDir()
	fs := map[string]any{
		"Downloads": map[string]any{
			"bar.torrent":     nil,
			"bar.txt":         nil,
			"foo.bar.torrent": nil,
			"foo.torrent":     nil,
			"foobar.torrent":  nil,
		},
		"bar.torrent": nil,
		"foo.torrent": nil,
	}
	prepareFs(dir, fs)
	cd(dir)
	t.Cleanup(func() { cd("") })
	tests := []struct {
		desc     string
		pattern  string
		expected []string
	}{
		{
			pattern:  "no_wildcard.txt",
			expected: nil,
		},
		{
			pattern:  "*.torrent",
			expected: []string{"./bar.torrent", "./foo.torrent"},
		},
		{
			pattern:  "*.txt",
			expected: []string{},
		},
		{
			pattern:  "Downloads/bar.*",
			expected: []string{"Downloads/bar.torrent", "Downloads/bar.txt"},
		},
		{
			pattern:  "Downloads/foo*.torrent",
			expected: []string{"Downloads/foo.bar.torrent", "Downloads/foo.torrent", "Downloads/foobar.torrent"},
		},
		{
			pattern:  "Downloads/foo.*.torrent",
			expected: []string{"Downloads/foo.bar.torrent"},
		},
		{
			desc:     "wildcard in dir is not supported",
			pattern:  "*/foo.torrent",
			expected: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			result := helper.GetWildcardFilenames(test.pattern)
			if !reflect.DeepEqual(test.expected, result) {
				t.Errorf("expected %v, got %v", test.expected, result)
			}
		})
	}
}

func cd(dir string) {
	if dir == "" {
		if homeDir, err := os.UserHomeDir(); err != nil {
			panic(err)
		} else {
			dir = homeDir
		}
	}
	if err := os.Chdir(dir); err != nil {
		panic(err)
	}
}

func prepareFs(dir string, fs map[string]any) {
	for name, entry := range fs {
		entryPath := filepath.Join(dir, name)
		if entry == nil {
			if f, err := os.Create(entryPath); err != nil {
				panic(err)
			} else {
				f.Close()
			}
		} else {
			if err := os.Mkdir(entryPath, 0600); err != nil {
				panic(err)
			}
			prepareFs(entryPath, entry.(map[string]any))
		}
	}
}
