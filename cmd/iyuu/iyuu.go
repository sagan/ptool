package iyuu

import (
	"sync"

	"github.com/glebarez/sqlite"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
)

// gorm "torrents" table
type Torrent struct {
	InfoHash       string `gorm:"primaryKey"`
	Sid            int64  // iyuu site id
	Tid            int64  // torrent id of site
	TargetInfoHash string `gorm:"index"` // original (target) torrent info hash
	// Site           Site   `gorm:"foreignKey:Sid;references:sid"` // do not use gorm associations feature
}

// gorm "sites" table
type Site struct {
	Sid          int64  `gorm:"primaryKey"`
	Name         string `gorm:"index"`
	Nickname     string
	Url          string // site homepage url. eg. https://hdvideo.one/
	DownloadPage string // (relative) torrent download url. eg. "download.php?id={}&passkey={passkey}"
}

// gorm "meta" (not metas!) table
type Meta struct {
	Key   string `gorm:"primaryKey"` // keys: lastUpdateTime
	Value string
}

var (
	db *gorm.DB
	mu sync.Mutex
)
var Command = &cobra.Command{
	Use:   "iyuu",
	Short: "Cross seed automation tool using iyuu API.",
	Long:  `Cross seed automation tool using iyuu API.`,
}

func init() {
	cmd.RootCmd.AddCommand(Command)
}

func Db() *gorm.DB {
	if db != nil {
		return db
	}
	mu.Lock()
	defer mu.Unlock()
	if db != nil {
		return db
	}
	dbfile := config.ConfigDir + "/iyuu.db"
	log.Tracef("iyuu open db file %s", dbfile)
	_db, err := gorm.Open(sqlite.Open(dbfile), &gorm.Config{})
	if err != nil {
		log.Fatalf("error create iyuu sqldb: %v", err)
	}
	err = _db.AutoMigrate(&Site{}, &Torrent{}, &Meta{})
	if err != nil {
		log.Fatalf("iyuu sql schema init error: %v", err)
	}

	db = _db
	return db
}
