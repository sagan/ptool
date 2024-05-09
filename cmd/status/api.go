package status

import (
	"fmt"
	"sort"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd/brush/strategy"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
)

type StatusResponse struct {
	Name              string
	Kind              int64
	ClientStatus      *client.Status
	ClientTorrents    []*client.Torrent
	SiteStatus        *site.Status
	SiteTorrents      []*site.Torrent // latest site torrents
	SiteTorrentScores map[string]float64
	Error             error
}

func fetchClientStatus(clientInstance client.Client, showTorrents bool, showAllTorrents bool,
	category string, ch chan *StatusResponse) {
	response := &StatusResponse{Name: clientInstance.GetName(), Kind: 1}

	clientStatus, err := clientInstance.GetStatus()
	response.ClientStatus = clientStatus
	if err != nil {
		response.Error = fmt.Errorf("cann't get client %s status: error=%w", clientInstance.GetName(), err)
		ch <- response
		return
	}

	if showTorrents {
		clientTorrents, err := clientInstance.GetTorrents("", category, showAllTorrents)
		if showAllTorrents {
			sort.Slice(clientTorrents, func(i, j int) bool {
				if clientTorrents[i].Name != clientTorrents[j].Name {
					return clientTorrents[i].Name < clientTorrents[j].Name
				}
				return clientTorrents[i].InfoHash < clientTorrents[j].InfoHash
			})
		} else {
			sort.Slice(clientTorrents, func(i, j int) bool {
				return clientTorrents[i].Atime > clientTorrents[j].Atime
			})
		}
		response.ClientTorrents = clientTorrents
		if err != nil {
			response.Error = fmt.Errorf("cann't get client %s torrents: %w", clientInstance.GetName(), err)
		}
	}
	ch <- response
}

func fetchSiteStatus(siteInstance site.Site, showTorrents bool, full bool, showScore bool, ch chan *StatusResponse) {
	response := &StatusResponse{Name: siteInstance.GetName(), Kind: 2}
	// if siteInstance.GetSiteConfig().Dead {
	// 	response.Error = fmt.Errorf("skip site %s: site is dead", siteInstance.GetName())
	// 	ch <- response
	// 	return
	// }
	SiteStatus, err := siteInstance.GetStatus()
	response.SiteStatus = SiteStatus
	if err != nil {
		response.Error = fmt.Errorf("cann't get site %s status: error=%w", siteInstance.GetName(), err)
		ch <- response
		return
	}

	if showTorrents {
		siteTorrents, err := siteInstance.GetLatestTorrents(full)
		if err != nil {
			response.Error = fmt.Errorf("cann't get site %s torrents: %w", siteInstance.GetName(), err)
		} else {
			if showScore {
				brushSiteOption := strategy.GetBrushSiteOptions(siteInstance, util.Now())
				scores := map[string]float64{}
				for _, torrent := range siteTorrents {
					scores[torrent.Id], _, _ = strategy.RateSiteTorrent(torrent, brushSiteOption)
				}
				response.SiteTorrentScores = scores
			}
			response.SiteTorrents = siteTorrents
		}
	}

	ch <- response
}
