// Package badges scrapes steamcommunity.com badge pages for card drop info.
package badges

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var (
	ErrAuth      = errors.New("Steam session is not logged in or expired")
	ErrRateLimit = errors.New("Steam rate limit (HTTP 429)")
)

type Game struct {
	AppID     uint32  `json:"appId"`
	Name      string  `json:"name"`
	Remaining int     `json:"remaining"` // card drops remaining
	Hours     float64 `json:"hours"`     // hours on record
}

var (
	reRemaining = regexp.MustCompile(`(?i)(\d+)\s+card drops?\s+remaining`)
	reHours     = regexp.MustCompile(`(?i)([\d,.]+)\s*hrs? on record`)
	reApp       = regexp.MustCompile(`/gamecards/(\d+)`)
)

// Client fetches badge pages using the given cookies.
type Client struct {
	http    *http.Client
	steamID string
	Delay   time.Duration // pause between pages
}

func NewClient(steamID string, cookies []*http.Cookie) *Client {
	jar, _ := cookiejar.New(nil)
	u, _ := url.Parse("https://steamcommunity.com/")
	jar.SetCookies(u, cookies)
	return &Client{
		http:    &http.Client{Jar: jar, Timeout: 40 * time.Second},
		steamID: steamID,
		Delay:   2 * time.Second,
	}
}

// FetchAll walks every badge page and returns games with drops remaining > 0
// (games with 0 remaining are skipped) plus the total number of badge rows.
func (c *Client) FetchAll(ctx context.Context) ([]Game, error) {
	var out []Game
	seen := map[uint32]bool{}
	for p := 1; p <= 200; p++ {
		games, rows, err := c.page(ctx, p)
		if err != nil {
			return nil, err
		}
		if rows == 0 {
			break
		}
		fresh := 0
		for _, g := range games {
			if !seen[g.AppID] {
				seen[g.AppID] = true
				fresh++
				if g.Remaining > 0 {
					out = append(out, g)
				}
			}
		}
		if fresh == 0 {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(c.Delay):
		}
	}
	return out, nil
}

func (c *Client) page(ctx context.Context, p int) (games []Game, rows int, err error) {
	u := fmt.Sprintf("https://steamcommunity.com/profiles/%s/badges/?l=english&sort=a&p=%d", c.steamID, p)
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) steam-card-xmk")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == 429:
		return nil, 0, ErrRateLimit
	case resp.StatusCode == 401 || resp.StatusCode == 403 || strings.Contains(resp.Request.URL.Path, "/login"):
		return nil, 0, ErrAuth
	case resp.StatusCode != 200:
		return nil, 0, fmt.Errorf("badges page %d: HTTP %d", p, resp.StatusCode)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	games, rows = ParseDoc(doc)
	if rows == 0 && p == 1 && doc.Find("#global_action_menu, .profile_small_header_name").Length() == 0 {
		return nil, 0, ErrAuth
	}
	return games, rows, nil
}

// ParseDoc extracts games from one badges page; rows is the number of badge rows seen.
func ParseDoc(doc *goquery.Document) (games []Game, rows int) {
	doc.Find(".badge_row").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Find("a.badge_row_overlay").Attr("href")
		m := reApp.FindStringSubmatch(href)
		if m == nil {
			return // e.g. special badges without an app
		}
		rows++
		id, _ := strconv.ParseUint(m[1], 10, 32)
		g := Game{AppID: uint32(id)}
		title := s.Find(".badge_title").First().Clone()
		title.Find("*").Remove()
		g.Name = strings.TrimSpace(strings.ReplaceAll(title.Text(), "\u00a0", " "))
		if rm := reRemaining.FindStringSubmatch(s.Find(".progress_info_bold").Text()); rm != nil {
			g.Remaining, _ = strconv.Atoi(rm[1])
		}
		if hm := reHours.FindStringSubmatch(s.Find(".badge_title_stats_playtime").Text()); hm != nil {
			g.Hours, _ = strconv.ParseFloat(strings.ReplaceAll(hm[1], ",", ""), 64)
		}
		games = append(games, g)
	})
	return
}
