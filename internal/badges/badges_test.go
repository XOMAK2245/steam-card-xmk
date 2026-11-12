package badges

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

const sample = `<div class="badge_row is_link"><div class="badge_row_inner">
<div class="badge_title_stats"><div class="badge_title_stats_content"><div class="badge_title_stats_drops">
<span class="progress_info_bold">3 card drops remaining</span></div>
<div class="badge_title_stats_playtime">
 1,234.5 hrs on record
</div></div></div>
<div class="badge_title">Half-Life&nbsp;<span class="badge_view_details">View details</span></div>
</div><a class="badge_row_overlay" href="https://steamcommunity.com/id/x/gamecards/70/"></a></div>
<div class="badge_row"><div class="badge_title">Done game</div>
<div class="badge_title_stats_playtime">0.5 hrs on record</div>
<a class="badge_row_overlay" href="https://steamcommunity.com/id/x/gamecards/220/"></a></div>`

func TestParse(t *testing.T) {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(sample))
	g, rows := ParseDoc(doc)
	if rows != 2 || len(g) != 2 {
		t.Fatalf("rows=%d games=%v", rows, g)
	}
	if g[0].AppID != 70 || g[0].Name != "Half-Life" || g[0].Remaining != 3 || g[0].Hours != 1234.5 {
		t.Errorf("bad parse: %+v", g[0])
	}
	if g[1].Remaining != 0 || g[1].Hours != 0.5 {
		t.Errorf("bad parse: %+v", g[1])
	}
}
