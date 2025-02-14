package tidyup

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/ptool/client"
	"github.com/sagan/ptool/cmd"
	"github.com/sagan/ptool/constants"
	"github.com/sagan/ptool/site/public"
	"github.com/sagan/ptool/site/tpl"
	"github.com/sagan/ptool/util"
)

var command = &cobra.Command{
	Use:         "tidyup {client}",
	Annotations: map[string]string{"cobra-prompt-dynamic-suggestions": "tidyup"},
	Short:       "Tidy up all torrents of client.",
	Long: `Tidy up all torrents of client.
Set appropriate tags to all torrents of a client.
For example, it will set the "site:m-team" tag for torrents downloaded from M-Team.

Note it will NOT change the category of torrents in client.
To set category of torrents to it's site, run "setcategory" cmd after running this cmd. E.g.
  ptool setcategory local mteam --tag site:mteam -vvv
The above cmd changes the category of all torrents in "local" client that has tag "site:mteam" to "mteam".`,
	Args: cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: tidyup,
}

var (
	dryRun      = false
	filter      = ""
	category    = ""
	tag         = ""
	maxTorrents = int64(0)
)

func init() {
	command.Flags().StringVarP(&filter, "filter", "", "", constants.HELP_ARG_FILTER_TORRENT)
	command.Flags().StringVarP(&category, "category", "", "", constants.HELP_ARG_CATEGORY)
	command.Flags().StringVarP(&tag, "tag", "", "", constants.HELP_ARG_TAG)
	command.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Dry run. Do NOT actually modify torrents to client")
	command.Flags().Int64VarP(&maxTorrents, "max-torrents", "", -1, "Number limit of modified torrents. -1 == no limit")
	cmd.RootCmd.AddCommand(command)
}

func tidyup(cmd *cobra.Command, args []string) error {
	clientName := args[0]
	clientInstance, err := client.CreateClient(clientName)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	torrents, err := client.QueryTorrents(clientInstance, category, tag, filter)
	if err != nil {
		return fmt.Errorf("failed to get torrents: %w", err)
	}
	domainSiteMap := map[string]string{}
	cntTorrents := int64(0)
	cntSuccessTorrents := int64(0)

	for i, torrent := range torrents {
		addTags := []string{}
		remopveTags := []string{}
		domain := util.GetUrlDomain(torrent.Tracker)
		log.Tracef("Torrent %s - %s: domain=%s", torrent.InfoHash, torrent.Name, domain)
		if domain != "" {
			sitename := ""
			ok := false
			if sitename, ok = domainSiteMap[domain]; !ok {
				if domainSiteMap[domain], err = tpl.GuessSiteByDomain(domain, ""); err == nil {
					sitename = domainSiteMap[domain]
				} else if site := public.GetSiteByDomain(domain); site != nil {
					sitename = site.Name
				} else {
					log.Warnf("Failed to find match site for %s: %v", domain, err)
				}
			}
			if sitename != "" {
				existingSitename := torrent.GetSiteFromTag()
				if existingSitename != "" && existingSitename != sitename {
					remopveTags = append(remopveTags, client.GenerateTorrentTagFromSite(existingSitename))
				}
				tag := client.GenerateTorrentTagFromSite(sitename)
				if !torrent.HasTag(tag) {
					addTags = append(addTags, tag)
				}
			}
		}
		if len(addTags) > 0 || len(remopveTags) > 0 {
			cntTorrents++
			if maxTorrents >= 0 && cntTorrents > maxTorrents {
				break
			}
			fmt.Printf("Modify (%d/%d) torrent %s - %s: addTags=%v; removeTags=%v\n", i+1, len(torrents),
				torrent.InfoHash, torrent.Name, addTags, remopveTags)
			if dryRun {
				continue
			}
			var err error
			var hasError bool
			err = clientInstance.AddTagsToTorrents([]string{torrent.InfoHash}, addTags)
			if err != nil {
				hasError = true
				log.Errorf("Failed to add tags: %v", err)
			}
			err = clientInstance.RemoveTagsFromTorrents([]string{torrent.InfoHash}, remopveTags)
			if err != nil {
				hasError = true
				log.Errorf("Failed to remove tags: %v", err)
			}
			if !hasError {
				cntSuccessTorrents++
			}
		}
	}
	fmt.Printf("Done tidying up %d torrents. Modify / success torrents = %d / %d\n",
		len(torrents), cntTorrents, cntSuccessTorrents)
	return nil
}
