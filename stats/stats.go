package stats

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"database/sql"

	_ "github.com/glebarez/go-sqlite"
	"github.com/sagan/ptool/config"
	"github.com/sagan/ptool/utils"
	log "github.com/sirupsen/logrus"
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
type StatDb struct {
	file   *os.File
	mu     sync.Mutex
	open   bool
	loaded bool
	sqldb  *sql.DB
}

var (
	Db *StatDb
)

func (db *StatDb) openStatFile() {
	if db.open {
		return
	}
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.open {
		return
	}
	db.open = true
	filepath := config.ConfigDir + "/ptool_stats.txt"
	log.Tracef("Brush open stats file: %s", filepath)
	f, err := os.OpenFile(filepath, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0777)
	if err != nil {
		log.Errorf("Brush failed to open stats file: %v", err)
	} else {
		db.file = f
	}
}

func (db *StatDb) AddTorrentStat(ts int64, event int64, torrentStat *TorrentStat) {
	db.openStatFile()

	if db.file != nil {
		buf := bytes.NewBuffer(nil)
		json.NewEncoder(buf).Encode(Stat{
			Ts:    ts,
			Event: event,
			Data:  torrentStat,
		})
		db.mu.Lock()
		db.file.Write(buf.Bytes())
		db.mu.Unlock()
	}
}

func (db *StatDb) ShowTrafficStats(client string) {
	db.prepare()

	now := utils.Now()
	var rows *sql.Rows
	sql := ""
	today := utils.FormatDate2(now)
	yesterday := utils.FormatDate2(now - 86400)
	yesterdayMinus7day := utils.FormatDate2(now - 86400*8)
	yesterdayMinus30day := utils.FormatDate2(now - 86400*31)
	timespans := []struct {
		name     string
		startday string
		endday   string
	}{
		{"<all time>", "", ""},
		{"last 30d", yesterdayMinus30day, yesterday},
		{"last 7d", yesterdayMinus7day, yesterday},
		{"yesterday", yesterday, yesterday},
		{"today", today, today},
	}

	clients := []string{}
	sql = "select DISTINCT client from torrent_traffics order by day desc limit 1000"
	rows, _ = db.sqldb.Query(sql, client)
	for rows.Next() {
		var client string
		rows.Scan(&client)
		if client != "" {
			clients = append(clients, client)
		}
	}
	rows.Close()
	if len(clients) > 5 {
		clients = clients[:5]
	}

	if client == "" {
		fmt.Printf("%20s:  ", " time \\ clients")
		for _, client := range clients {
			fmt.Printf("%25s  /  ", client+"(↓, ↑)")
		}
		fmt.Printf("%25s\n", "<all>")
		for _, timespan := range timespans {
			sql = `SELECT client, ifnull(sum(downloaded),0), ifnull(sum(uploaded),0) from torrent_traffics where 1=1`
			if timespan.startday != "" {
				sql += " and day >= '" + timespan.startday + "'"
			}
			if timespan.endday != "" {
				sql += " and day <= '" + timespan.endday + "'"
			}
			sql += " group by client"

			rows, _ := db.sqldb.Query(sql)
			allDownloaded := int64(0)
			allUploaded := int64(0)
			clientsDownloaded := make(map[string](int64))
			clientsUploaded := make(map[string](int64))
			for rows.Next() {
				var client string
				var downloaded int64
				var uploaded int64
				rows.Scan(&client, &downloaded, &uploaded)
				if client != "" {
					clientsDownloaded[client] = downloaded
					clientsUploaded[client] = uploaded
				}
				allDownloaded += downloaded
				allUploaded += uploaded
			}
			fmt.Printf("%20s:  ", timespan.name)
			for _, client := range clients {
				fmt.Printf("%25s  /  ", "↓"+utils.BytesSize(float64(clientsDownloaded[client]))+", ↑"+utils.BytesSize(float64(clientsUploaded[client])))
			}
			fmt.Printf("%25s\n", "↓"+utils.BytesSize(float64(allDownloaded))+", ↑"+utils.BytesSize(float64(allUploaded)))
			rows.Close()
		}
		return
	}

	sites := []string{}
	sql = "select DISTINCT site from torrent_traffics where client = ? order by day desc limit 1000"
	rows, _ = db.sqldb.Query(sql, client)
	for rows.Next() {
		var site string
		rows.Scan(&site)
		if site != "" {
			sites = append(sites, site)
		}
	}
	rows.Close()
	if len(sites) > 5 {
		sites = sites[:5]
	}

	fmt.Printf("%20s:  ", client+" \\ sites")
	for _, site := range sites {
		fmt.Printf("%25s  /  ", site+"(↓, ↑)")
	}
	fmt.Printf("%25s\n", "<all>")
	for _, timespan := range timespans {
		sql = `SELECT site, ifnull(sum(downloaded),0), ifnull(sum(uploaded),0) from torrent_traffics where client = ?`
		if timespan.startday != "" {
			sql += " and day >= '" + timespan.startday + "'"
		}
		if timespan.endday != "" {
			sql += " and day <= '" + timespan.endday + "'"
		}
		sql += " group by site"

		rows, _ := db.sqldb.Query(sql, client)
		allDownloaded := int64(0)
		allUploaded := int64(0)
		sitesDownloaded := make(map[string](int64))
		siteUploaded := make(map[string](int64))
		for rows.Next() {
			var site string
			var downloaded int64
			var uploaded int64
			rows.Scan(&site, &downloaded, &uploaded)
			if site != "" {
				sitesDownloaded[site] = downloaded
				siteUploaded[site] = uploaded
			}
			allDownloaded += downloaded
			allUploaded += uploaded
		}
		fmt.Printf("%20s:  ", timespan.name)
		for _, site := range sites {
			fmt.Printf("%25s  /  ", "↓"+utils.BytesSize(float64(sitesDownloaded[site]))+", ↑"+utils.BytesSize(float64(siteUploaded[site])))
		}
		fmt.Printf("%25s\n", "↓"+utils.BytesSize(float64(allDownloaded))+", ↑"+utils.BytesSize(float64(allUploaded)))
		rows.Close()
	}
}

func (db *StatDb) querySingleRow(sql string, variables ...any) error {
	rows, err := db.sqldb.Query(sql)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		return err
	}
	if err := rows.Scan(variables...); err != nil {
		return err
	}
	return nil
}

func (db *StatDb) prepare() {
	if db.sqldb != nil {
		return
	}
	db.openStatFile()
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.sqldb != nil {
		return
	}
	batch := []string{
		`CREATE TABLE torrent_traffics (
			client varchar, day varchar, site varchar, downloaded int, uploaded int,
			primary key(client, day, site)
		);`,
	}
	_db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		log.Fatalf("error create stats sqldb: %v", err)
	}
	for _, b := range batch {
		_, err = _db.Exec(b)
		if err != nil {
			log.Fatalf("error initialize stats sqldb: %v", err)
		}
	}
	db.sqldb = _db

	if db.file == nil {
		return
	}
	db.file.Seek(0, 0)
	fileScanner := bufio.NewScanner(db.file)
	fileScanner.Split(bufio.ScanLines)

	for fileScanner.Scan() {
		statRecord := Stat{}
		err := json.Unmarshal([]byte(fileScanner.Text()), &statRecord)
		if err != nil || statRecord.Event != 1 {
			continue
		}
		_, err = db.sqldb.Exec(
			`INSERT INTO torrent_traffics (client, day, site, downloaded, uploaded) VALUES (?,?,?,?,?)
			ON CONFLICT(client, day, site) DO UPDATE SET downloaded = downloaded + ?, uploaded = uploaded + ?;
			`,
			statRecord.Data.Client, utils.FormatDate2(statRecord.Ts), statRecord.Data.Site, statRecord.Data.Downloaded, statRecord.Data.Uploaded,
			statRecord.Data.Downloaded, statRecord.Data.Uploaded,
		)
		if err != nil {
			log.Tracef("StatDb.addAlll insert error: %s", err)
		}
	}
}

func init() {
	Db = &StatDb{}
}
