package cmd

import (
	"os"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/sagan/ptool/config"
)

var (
	historyFile       *os.File
	historyFileOpened bool
	mu                sync.Mutex
)

func WriteHistory(in string) {
	if !historyFileOpened {
		mu.Lock()
		defer mu.Unlock()
		if !historyFileOpened {
			historyFileOpened = true
			filename := config.ConfigDir + "/" + config.HISTORY_FILENAME
			file, err := os.OpenFile(filename, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0777)
			if err != nil {
				log.Debugf("Failed to open history file %s: %v", filename, err)
			} else {
				log.Debugf("History file %s opened", filename)
				historyFile = file
			}
		}
	}
	if historyFile != nil {
		_, err := historyFile.WriteString(in + "\n")
		if err != nil {
			log.Tracef("Failed to write history file: %v", err)
		}
	}
}
