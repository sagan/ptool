package status

import (
	"fmt"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd/brush/strategy"
	"github.com/sagan/ptool/site"
	"github.com/sagan/ptool/util"
)

type StatusResponse struct {
	Name              string
	Kind              int64
	ClientStatus      *client.Status
	ClientTorrents    []client.Torrent
	SiteStatus        *site.Status
	SiteTorrents      []site.Torrent // latest site torrents
	SiteTorrentScores map[string]float64
	Error             error
}

// side effect for now: close clientInstance before return
func fetchClientStatus(clientInstance client.Client, showTorrents bool, showAllTorrents bool,
	category string, ch chan *StatusResponse) {
	response := &StatusResponse{Name: clientInstance.GetName(), Kind: 1}

	clientStatus, err := clientInstance.GetStatus()
	response.ClientStatus = clientStatus
	if err != nil {
		response.Error = fmt.Errorf("cann't get client %s status: error=%v", clientInstance.GetName(), err)
		ch <- response
		return
	}

	if showTorrents {
		clientTorrents, err := clientInstance.GetTorrents("", category, showAllTorrents)
		response.ClientTorrents = clientTorrents
		if err != nil {
			response.Error = fmt.Errorf("cann't get client %s torrents: %v", clientInstance.GetName(), err)
		}
	}
	ch <- response
}

func fetchSiteStatus(siteInstance site.Site, showTorrents bool, full bool, showScore bool, ch chan *StatusResponse) {
	response := &StatusResponse{Name: siteInstance.GetName(), Kind: 2}

	SiteStatus, err := siteInstance.GetStatus()
	response.SiteStatus = SiteStatus
	if err != nil {
		response.Error = fmt.Errorf("cann't get site %s status: error=%v", siteInstance.GetName(), err)
		ch <- response
		return
	}

	if showTorrents {
		siteTorrents, err := siteInstance.GetLatestTorrents(full)
		if err != nil {
			response.Error = fmt.Errorf("cann't get site %s torrents: %v", siteInstance.GetName(), err)
		} else {
			if showScore {
				brushSiteOption := strategy.GetBrushSiteOptions(siteInstance, util.Now())
				scores := map[string]float64{}
				for _, torrent := range siteTorrents {
					scores[torrent.Id], _, _ = strategy.RateSiteTorrent(&torrent, brushSiteOption)
				}
				response.SiteTorrentScores = scores
			}
			response.SiteTorrents = siteTorrents
		}
	}

	ch <- response
}
