package stats

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/util"
)

type TorrentTraffic struct {
	Client     string `gorm:"primaryKey"`
	Day        string `gorm:"primaryKey"`
	Site       string `gorm:"primaryKey"`
	Downloaded int64
	Uploaded   int64
}

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
	statFilename string
	file         *os.File
	mu           sync.Mutex
	sqldb        *gorm.DB
}

func (db *StatDb) AddTorrentStat(ts int64, event int64, torrentStat *TorrentStat) {
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

func (db *StatDb) ShowTrafficStats(client string) {
	now := util.Now()
	today := util.FormatDate(now)
	yesterday := util.FormatDate(now - 86400)
	yesterdayMinus7day := util.FormatDate(now - 86400*8)
	yesterdayMinus30day := util.FormatDate(now - 86400*31)
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

	var clientObjs []TorrentTraffic
	db.sqldb.Distinct("client").Order("day desc").Limit(1000).Find(&clientObjs)
	sort.Slice(clientObjs, func(i, j int) bool {
		if clientObjs[i].Uploaded != clientObjs[j].Uploaded {
			return clientObjs[i].Uploaded > clientObjs[j].Uploaded
		}
		return clientObjs[i].Downloaded+clientObjs[i].Uploaded > clientObjs[j].Downloaded+clientObjs[j].Uploaded
	})
	if len(clientObjs) > 3 {
		clientObjs = clientObjs[:3]
	}
	clients := util.Map(clientObjs, func(c TorrentTraffic) string {
		return c.Client
	})

	if client == "" {
		fmt.Printf("%-15s  ", `time\clients`)
		for _, client := range clients {
			fmt.Printf("%20s  /  ", client+"(↓, ↑)")
		}
		fmt.Printf("%20s\n", "<all>")
		for _, timespan := range timespans {
			records := []TorrentTraffic{}
			tx := db.sqldb.Table("torrent_traffics").
				Select("client", "ifnull(sum(downloaded),0) as downloaded", "ifnull(sum(uploaded),0) as uploaded").Group("client")
			if timespan.startday != "" {
				tx = tx.Where("day >= ?", timespan.startday)
			}
			if timespan.endday != "" {
				tx = tx.Where("day <= ?", timespan.endday)
			}
			tx.Find(&records)
			allDownloaded := int64(0)
			allUploaded := int64(0)
			clientsDownloaded := map[string]int64{}
			clientsUploaded := map[string]int64{}
			for _, record := range records {
				if record.Client != "" {
					clientsDownloaded[record.Client] = record.Downloaded
					clientsUploaded[record.Client] = record.Uploaded
				}
				allDownloaded += record.Downloaded
				allUploaded += record.Uploaded
			}
			fmt.Printf("%-15s  ", timespan.name)
			for _, client := range clients {
				fmt.Printf("%20s  /  ", "↓"+util.BytesSize(float64(clientsDownloaded[client]))+", ↑"+util.BytesSize(float64(clientsUploaded[client])))
			}
			fmt.Printf("%20s\n", "↓"+util.BytesSize(float64(allDownloaded))+", ↑"+util.BytesSize(float64(allUploaded)))
		}
		return
	}

	var siteObjs []TorrentTraffic
	db.sqldb.Distinct("site").Where("site != ?", "").Order("day desc").Limit(1000).Find(&siteObjs)
	if len(siteObjs) > 3 {
		siteObjs = siteObjs[:3]
	}
	sort.Slice(siteObjs, func(i, j int) bool {
		if siteObjs[i].Uploaded != siteObjs[j].Uploaded {
			return siteObjs[i].Uploaded > siteObjs[j].Uploaded
		}
		return siteObjs[i].Downloaded+siteObjs[i].Uploaded > siteObjs[j].Downloaded+siteObjs[j].Uploaded
	})
	sites := util.Map(siteObjs, func(c TorrentTraffic) string {
		return c.Site
	})

	fmt.Printf("%-15s  ", client+`\sites`)
	for _, site := range sites {
		fmt.Printf("%20s  /  ", site+"(↓, ↑)")
	}
	fmt.Printf("%20s\n", "<all>")
	for _, timespan := range timespans {
		records := []TorrentTraffic{}
		tx := db.sqldb.Table("torrent_traffics").
			Select("site", "ifnull(sum(downloaded),0) downloaded", "ifnull(sum(uploaded),0) uploaded").
			Group("site").
			Where("client = ?", client)
		if timespan.startday != "" {
			tx = tx.Where("day >= ?", timespan.startday)
		}
		if timespan.endday != "" {
			tx = tx.Where("day <= ?", timespan.endday)
		}
		tx.Find(&records)

		allDownloaded := int64(0)
		allUploaded := int64(0)
		sitesDownloaded := map[string]int64{}
		siteUploaded := map[string]int64{}
		for _, record := range records {
			if record.Site != "" {
				sitesDownloaded[record.Site] = record.Downloaded
				siteUploaded[record.Site] = record.Uploaded
			}
			allDownloaded += record.Downloaded
			allUploaded += record.Uploaded
		}
		fmt.Printf("%-15s  ", timespan.name)
		for _, site := range sites {
			fmt.Printf("%20s  /  ", "↓"+util.BytesSize(float64(sitesDownloaded[site]))+", ↑"+util.BytesSize(float64(siteUploaded[site])))
		}
		fmt.Printf("%20s\n", "↓"+util.BytesSize(float64(allDownloaded))+", ↑"+util.BytesSize(float64(allUploaded)))
	}
}

func NewDb(statFilename string) (*StatDb, error) {
	db := &StatDb{}

	f, err := os.OpenFile(statFilename, os.O_RDWR|os.O_APPEND|os.O_CREATE, constants.PERM)
	if err != nil {
		return nil, fmt.Errorf("failed to open stats file %s: %v", statFilename, err)
	}
	db.statFilename = statFilename
	db.file = f

	sqldb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("error create stats sqldb: %v", err)
	}
	err = sqldb.AutoMigrate(&TorrentTraffic{})
	if err != nil {
		return nil, fmt.Errorf("sql schema init error: %v", err)
	}
	db.sqldb = sqldb
	db.file.Seek(0, 0)
	fileScanner := bufio.NewScanner(db.file)
	fileScanner.Split(bufio.ScanLines)

	flagMap := map[string]bool{}
	for fileScanner.Scan() {
		statRecord := Stat{}
		err := json.Unmarshal([]byte(fileScanner.Text()), &statRecord)
		if err != nil || statRecord.Event != 1 {
			continue
		}
		timespan := statRecord.Ts - statRecord.Data.Atime
		if timespan == 0 {
			continue // just skip it
		}
		id := fmt.Sprint(statRecord.Data.Client, statRecord.Data.InfoHash, statRecord.Data.Atime)
		if flagMap[id] {
			continue // duplicate records
		}
		flagMap[id] = true
		aDownloadSpeed := statRecord.Data.Downloaded / timespan
		aUploadSpeed := statRecord.Data.Uploaded / timespan
		dailyDownloaded := 86400 * aDownloadSpeed
		dailyUploaded := 86400 * aUploadSpeed

		time := statRecord.Data.Atime
		day := util.FormatDate(time)
		nexydayTime, _ := util.ParseLocalDateTime(day)
		nexydayTime += 86400
		for statRecord.Ts > time {
			isFullDay := true
			time2 := nexydayTime
			if time2 > statRecord.Ts {
				time2 = statRecord.Ts
				isFullDay = false
			} else if time == statRecord.Data.Atime {
				isFullDay = false
			}
			downloaded := int64(0)
			uploaded := int64(0)
			if isFullDay {
				downloaded = dailyDownloaded
				uploaded = dailyUploaded
			} else {
				downloaded = (time2 - time) * aDownloadSpeed
				uploaded = (time2 - time) * aUploadSpeed
			}
			// INSERT INTO torrent_traffics (client, day, site, downloaded, uploaded) VALUES (?,?,?,?,?)
			//	ON CONFLICT(client, day, site) DO UPDATE SET downloaded = downloaded + ?, uploaded = uploaded + ?;
			db.sqldb.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "client"}, {Name: "day"}, {Name: "site"}},
				DoUpdates: clause.Assignments(map[string]interface{}{
					"downloaded": gorm.Expr("downloaded + ?", downloaded),
					"uploaded":   gorm.Expr("uploaded + ?", uploaded),
				}),
			}).Create(&TorrentTraffic{
				Client:     statRecord.Data.Client,
				Day:        day,
				Site:       statRecord.Data.Site,
				Downloaded: downloaded,
				Uploaded:   uploaded,
			})
			time = nexydayTime
			day = util.FormatDate(time)
			nexydayTime += 86400
		}
	}
	return db, nil
}

func init() {
}
