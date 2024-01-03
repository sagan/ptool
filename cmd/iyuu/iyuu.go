package iyuu

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/glebarez/sqlite"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/util"
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
	Url          string // site homepage url. e.g.: https://hdvideo.one/
	DownloadPage string // (relative) torrent download url. e.g.: "download.php?id={}&passkey={passkey}"
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
	dbfile := filepath.Join(config.ConfigDir, "iyuu.db")
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

func (iyuuSite *Site) MatchFilter(filter string) bool {
	return filter == "" ||
		util.ContainsI(iyuuSite.Name, filter) ||
		util.ContainsI(iyuuSite.Nickname, filter) ||
		util.ContainsI(iyuuSite.Url, filter) ||
		fmt.Sprint(iyuuSite.Sid) == filter
}
