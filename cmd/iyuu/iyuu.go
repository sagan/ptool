package iyuu

import (
	"sync"

	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/config"
	"github.com/spf13/cobra"

	"github.com/glebarez/sqlite"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// gorm "torrents" table
type Torrent struct {
	InfoHash     string `gorm:"primaryKey"`
	SameInfoHash string `gorm:"unique"`
	Sid          int64  // iyuu site id
}

// gorm "sites" table
type Site struct {
	Sid  int64 `gorm:"primaryKey"`
	Name string
	Url  string
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
	_db, err := gorm.Open(sqlite.Open(dbfile), &gorm.Config{})
	if err != nil {
		log.Fatalf("error create iyuu sqldb: %v", err)
	}
	err = _db.AutoMigrate(&Site{}, &Torrent{})
	if err != nil {
		log.Fatalf("iyuu sql schema init error: %v", err)
	}

	db = _db
	return db
}
