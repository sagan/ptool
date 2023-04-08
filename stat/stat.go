package stat

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/sagan/ptool/config"
	log "github.com/sirupsen/logrus"
)

var (
	statMu   sync.Mutex
	statFile *os.File
	statOpen bool
)

type TorrentStat struct {
	Client     string `json:"client"`
	Site       string `json:"site"`
	Category   string `json:"category"`
	InfoHash   string `json:"infoHash"`
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	Atime      int64  `json:"atime"`
	Uploaded   int64  `json:"uploaded"`
	Downloaded int64  `json:"downloaded"`
	Msg        string `json:"msg"`
}

type Stat struct {
	Ts    int64        `json:"ts"`
	Event int64        `json:"event"`
	Data  *TorrentStat `json:"data"`
}

type Statistics struct {
	Downloaded int64
	Uploaded   int64
}

func openStatFile() {
	if !statOpen {
		statMu.Lock()
		if !statOpen {
			statOpen = true
			filepath := config.ConfigDir + "/ptool_stats.txt"
			log.Errorf("Brush open stats file: %s", filepath)
			f, err := os.OpenFile(filepath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0777)
			if err != nil {
				log.Errorf("Brush failed to open stats file: %v", err)
			} else {
				statFile = f
			}
		}
		statMu.Unlock()
	}
}

func AddTorrentStat(ts int64, event int64, torrentStat *TorrentStat) {
	openStatFile()
	if statFile != nil {
		buf := bytes.NewBuffer(nil)
		json.NewEncoder(buf).Encode(Stat{
			Ts:    ts,
			Event: event,
			Data:  torrentStat,
		})
		statMu.Lock()
		statFile.Write(buf.Bytes())
		statMu.Unlock()
	}
}

// [timespan][client][site]] timespan:  today, yesterday, 2020-01, 2020, all
func ShowTrafficStats(category string) *map[string](map[string](map[string](Statistics))) {
	openStatFile()
	if statFile == nil {
		return nil
	}
	result := make(map[string](map[string](map[string](Statistics))))
	statMu.Lock()
	defer statMu.Unlock()

	statFile.Seek(0, 0)
	fileScanner := bufio.NewScanner(statFile)
	fileScanner.Split(bufio.ScanLines)
	for fileScanner.Scan() {
		statRecord := Stat{}
		err := json.Unmarshal([]byte(fileScanner.Text()), &statRecord)
		if err != nil {
			continue
		}
		fmt.Println(statRecord.Ts, statRecord.Data.Name)
	}

	return &result
}
