package nexusphp

import (
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/sagan/ptool/site"
)

// opencdStyleHTML mimics the OpenCD torrents page: torrent title links use
// plugin_details.php (not the nexusphp default details.php), and the title
// text is wrapped in <b> inside the anchor.
const opencdStyleHTML = `<html><head><title>OpenCD :: 種子</title></head><body>
<table class="torrents"><tbody>
<tr><td>類型</td><td>標題</td><td>大小</td><td>时间</td><td>種子</td><td>下載</td><td>完成</td></tr>
<tr>
	<td><a href="download.php?id=111">DL</a></td>
	<td><a title="Artist A - Album 2020" href="plugin_details.php?id=111&amp;hit=1"><b>Artist A - Album 2020</b></a></td>
	<td>100MB</td><td>2024-01-01 00:00:00</td><td>5</td><td>2</td><td>10</td>
</tr>
<tr>
	<td><a href="download.php?id=222">DL</a></td>
	<td><a title="Artist B - Single 2021" href="plugin_details.php?id=222&amp;hit=1"><b>Artist B - Single 2021</b></a></td>
	<td>50MB</td><td>2024-02-02 00:00:00</td><td>3</td><td>1</td><td>7</td>
</tr>
</tbody></table></body></html>`

func parseOpencdStyleHTML(t *testing.T, detailsLinkSelector string) []*site.Torrent {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(opencdStyleHTML))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	option := &TorrentsParserOption{
		location:                   time.Local,
		siteurl:                    "https://open.cd/",
		selectorTorrentDetailsLink: detailsLinkSelector,
	}
	torrents, err := parseTorrents(doc, option, 0, "opencd")
	if err != nil {
		t.Fatalf("parseTorrents error: %v", err)
	}
	return torrents
}

// With the OpenCD-specific selector, torrent names are extracted.
func TestParseTorrentsOpenCDStyleSelector(t *testing.T) {
	torrents := parseOpencdStyleHTML(t, `a[href^="plugin_details.php?"]`)
	if len(torrents) != 2 {
		t.Fatalf("expected 2 torrents, got %d", len(torrents))
	}
	wantNames := map[string]bool{"Artist A - Album 2020": false, "Artist B - Single 2021": false}
	for _, tr := range torrents {
		if tr.Name == "" {
			t.Errorf("torrent %+v has empty name", tr)
			continue
		}
		if _, ok := wantNames[tr.Name]; !ok {
			t.Errorf("unexpected torrent name %q", tr.Name)
		}
		wantNames[tr.Name] = true
		if tr.Id == "" {
			t.Errorf("torrent %q has empty id", tr.Name)
		}
		if tr.DownloadUrl == "" {
			t.Errorf("torrent %q has empty download url", tr.Name)
		}
	}
	for name, found := range wantNames {
		if !found {
			t.Errorf("expected torrent name %q not parsed", name)
		}
	}
}

// With the nexusphp default details.php selector, OpenCD-style pages yield no
// torrents because title links (plugin_details.php) don't match — demonstrating
// the bug this change fixes.
func TestParseTorrentsOpenCDStyleDefaultSelectorYieldsNone(t *testing.T) {
	torrents := parseOpencdStyleHTML(t, "") // empty -> default details.php selector
	if len(torrents) != 0 {
		t.Fatalf("expected 0 torrents with default selector, got %d: %+v", len(torrents), torrents)
	}
}
