package qbittorrent

import (
	"fmt"
	"os"

	"github.com/anacrolix/torrent/bencode"
)

type FastresumeFile struct {
	Trackers [][]string `bencode:"trackers,omitempty"`
}

func parseQbFastresumeFile(path string) (*FastresumeFile, error) {
	fd, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	var ff FastresumeFile
	d := bencode.NewDecoder(fd)
	err = d.Decode(&ff)
	if err != nil {
		return nil, err
	}
	err = d.ReadEOF()
	if err != nil {
		return nil, fmt.Errorf("error after decoding bencode: %w", err)
	}
	return &ff, nil
}
