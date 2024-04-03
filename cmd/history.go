package cmd

import (
	"io"
	"os"
	"strings"
	"sync"

	"github.com/gofrs/flock"
	"github.com/sagan/ptool/constants"
	log "github.com/sirupsen/logrus"
)

// direct construct, only filename field needed
type ShellHistoryStruct struct {
	filename string
	cnt      int
	opened   bool
	file     *os.File
	mu       sync.Mutex
}

func (sh *ShellHistoryStruct) reset() {
	sh.opened = false
	if sh.file != nil {
		sh.file.Close()
		sh.file = nil
	}
	sh.cnt = 0
}

func (sh *ShellHistoryStruct) openHistoryFile() {
	if !sh.opened {
		sh.opened = true
		file, err := os.OpenFile(sh.filename, os.O_RDWR|os.O_APPEND|os.O_CREATE, constants.PERM)
		if err != nil {
			log.Debugf("Failed to open history file %s: %v", sh.filename, err)
		} else {
			log.Debugf("History file %s opened", sh.filename)
			sh.file = file
		}
	}
}

func (sh *ShellHistoryStruct) readHistoryFile() ([]string, error) {
	sh.file.Seek(0, 0)
	historyData, err := io.ReadAll(sh.file)
	log.Tracef("read history from file %s, err=%v", sh.filename, err)
	if err != nil {
		return nil, err
	}
	history := strings.Split(string(historyData), "\n")
	if history[len(history)-1] == "" {
		history = history[:len(history)-1] // remove last empty new line
	}
	sh.cnt = len(history)
	return history, nil
}

func (sh *ShellHistoryStruct) Write(in string) {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.openHistoryFile()
	if sh.file == nil {
		return
	}
	_, err := sh.file.WriteString(in + "\n")
	if err != nil {
		log.Tracef("Failed to write history file: %v", err)
	} else {
		sh.cnt++
	}
}

func (sh *ShellHistoryStruct) Load() ([]string, error) {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.openHistoryFile()
	if sh.file == nil {
		return nil, nil
	}
	return sh.readHistoryFile()
}

func (sh *ShellHistoryStruct) Clear() {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	sh.openHistoryFile()
	if sh.file == nil {
		return
	}
	log.Tracef("Truncate history file: %s", sh.filename)
	// sh.file.Truncate(0) // this does NOT work for files opened with O_APPEND
	sh.reset()
	os.Remove(sh.filename)
}

// best effort
func (sh *ShellHistoryStruct) Truncate(max int) {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	if max < 0 || sh.cnt <= max {
		return
	}
	sh.openHistoryFile()
	if sh.file == nil {
		return
	}
	if max == 0 {
		sh.reset()
		os.Remove(sh.filename)
	} else { // cnt > max > 0, re-write history file
		ok, err := flock.New(sh.filename).TryLock()
		if err != nil || !ok {
			return
		}
		history, err := sh.readHistoryFile()
		if err != nil || len(history) <= max {
			return
		}
		sh.reset()
		history = history[len(history)-max:]
		historyData := strings.Join(history, "\n")
		os.WriteFile(sh.filename, []byte(historyData), constants.PERM)
		sh.cnt = len(history)
	}
}
