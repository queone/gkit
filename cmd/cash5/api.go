package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/queone/gkit/internal/color"

	"golang.org/x/net/html"
)

const baseURL = "https://www.njlottery.com/api/v1/draw-games/draws/page"

// checkInternet returns true if a network connection can be established.
// Uses a TCP dial to Cloudflare DNS — fast, reliable, no HTTP overhead.
func checkInternet() bool {
	conn, err := net.DialTimeout("tcp", "1.1.1.1:53", 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// DrawFetcher is the interface for any source that can return NJ Cash 5 draws.
type DrawFetcher interface {
	Name() string
	FetchRecent(dateFrom, dateTo time.Time) ([]Draw, error)
}

// backupFetchers is the ordered fallback chain tried when the primary source fails.
// Only lottonumbers.com responds to plain HTTP GET; lotterypost.com is behind
// Cloudflare and lotteryusa.com returns 403.
var backupFetchers = []DrawFetcher{
	&lottoNumbersFetcher{},
}

// --- Backup fetcher: new-jersey.lottonumbers.com ---
// Scrapes the HTML results page. No API — data is SSR'd directly into the HTML.
// lotterypost.com is blocked by Cloudflare; lotteryusa.com returns 403.

type lottoNumbersFetcher struct{}

func (f *lottoNumbersFetcher) Name() string { return "new-jersey.lottonumbers.com" }

func (f *lottoNumbersFetcher) FetchRecent(dateFrom, dateTo time.Time) ([]Draw, error) {
	const targetURL = "https://new-jersey.lottonumbers.com/cash-5/results"

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: request failed: %w", f.Name(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("%s: bad status %s", f.Name(), resp.Status)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s: HTML parse failed: %w", f.Name(), err)
	}

	draws, err := parseLottoNumbersDraws(doc)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", f.Name(), err)
	}

	// Filter to requested date range
	var filtered []Draw
	for _, d := range draws {
		t := time.UnixMilli(d.DrawTime)
		if (t.Equal(dateFrom) || t.After(dateFrom)) && (t.Equal(dateTo) || t.Before(dateTo)) {
			filtered = append(filtered, d)
		}
	}

	return filtered, nil
}

// parseLottoNumbersDraws extracts draw data from the lottonumbers.com HTML tree.
//
// Target structure (repeated per draw):
//
//	<div class="draw">
//	  <div class="resultBox">
//	    <div><strong>Saturday</strong><br>February 21, 2026</div>
//	    <ul class="balls multiplier">
//	      <li class="ball ball">14</li>       ← regular numbers (4 of these)
//	      <li class="ball bullseye">20</li>   ← 5th number / bullseye
//	      <li class="ball xtra-number">3</li> ← Xtra multiplier, skip
//	    </ul>
//	  </div>
//	  <div class="resultBoxStats">
//	    <p>Jackpot: <strong>$764,968</strong></p>
//	  </div>
//	</div>
func parseLottoNumbersDraws(doc *html.Node) ([]Draw, error) {
	var draws []Draw

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && hasClass(n, "draw") {
			d, err := parseSingleLottoNumbersDraw(n)
			if err == nil {
				draws = append(draws, d)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	if len(draws) == 0 {
		return nil, fmt.Errorf("no draws found in page — site structure may have changed")
	}

	return draws, nil
}

func parseSingleLottoNumbersDraw(drawNode *html.Node) (Draw, error) {
	var d Draw
	d.GameName = "Cash 5"

	// Locate resultBox and resultBoxStats child divs
	var resultBox, resultBoxStats *html.Node
	for c := drawNode.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "div" {
			if hasClass(c, "resultBox") {
				resultBox = c
			} else if hasClass(c, "resultBoxStats") {
				resultBoxStats = c
			}
		}
	}
	if resultBox == nil {
		return d, fmt.Errorf("no resultBox found")
	}

	// Extract date from first child div of resultBox.
	// Structure: <div><strong>Saturday</strong><br>February 21, 2026</div>
	// The bare text node after <br> is the date string we want.
	dateText := ""
	for c := resultBox.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "div" {
			for gc := c.FirstChild; gc != nil; gc = gc.NextSibling {
				if gc.Type == html.TextNode {
					if t := strings.TrimSpace(gc.Data); t != "" {
						dateText = t
					}
				}
			}
			break
		}
	}
	if dateText == "" {
		return d, fmt.Errorf("date text not found")
	}

	t, err := time.ParseInLocation("January 2, 2006", dateText, easternTime())
	if err != nil {
		return d, fmt.Errorf("cannot parse date %q: %w", dateText, err)
	}
	// Draw time is 10:57 PM ET
	t = time.Date(t.Year(), t.Month(), t.Day(), 22, 57, 0, 0, easternTime())
	d.DrawTime = t.UnixMilli()
	d.ID = fmt.Sprintf("lottonumbers-%s", t.Format("2006-01-02"))

	// Extract numbers from <ul class="balls multiplier">.
	// Skip <li class="... xtra-number"> — that's the Xtra multiplier, not a draw number.
	var numbers []string
	ulNode := findFirstElement(resultBox, "ul")
	if ulNode == nil {
		return d, fmt.Errorf("balls ul not found")
	}
	for li := ulNode.FirstChild; li != nil; li = li.NextSibling {
		if li.Type != html.ElementNode || li.Data != "li" {
			continue
		}
		if hasClass(li, "xtra-number") {
			continue
		}
		if text := strings.TrimSpace(nodeText(li)); text != "" {
			numbers = append(numbers, text)
		}
	}
	if len(numbers) < 5 {
		return d, fmt.Errorf("expected 5 numbers, got %d", len(numbers))
	}
	d.Results = []Result{{Primary: numbers[:5]}}

	// Extract jackpot from resultBoxStats if present.
	// <p>Jackpot: <strong>$764,968</strong></p>
	if resultBoxStats != nil {
		var findJackpot func(*html.Node)
		findJackpot = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "strong" {
				if text := strings.TrimSpace(nodeText(n)); strings.HasPrefix(text, "$") {
					s := strings.ReplaceAll(strings.TrimPrefix(text, "$"), ",", "")
					if dollars, err := strconv.ParseInt(s, 10, 64); err == nil {
						d.EstimatedJackpot = dollars * 100
					}
				}
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				findJackpot(c)
			}
		}
		findJackpot(resultBoxStats)
	}

	return d, nil
}

// easternTime returns the US/Eastern timezone, falling back to UTC.
func easternTime() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.UTC
	}
	return loc
}

// hasClass reports whether an HTML node has the given CSS class.
func hasClass(n *html.Node, class string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" && slices.Contains(strings.Fields(a.Val), class) {
			return true
		}
	}
	return false
}

// nodeText returns the concatenated text content of a node and its descendants.
func nodeText(n *html.Node) string {
	var sb strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return sb.String()
}

// findFirstElement does a depth-first search for the first element with the given tag.
func findFirstElement(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findFirstElement(c, tag); found != nil {
			return found
		}
	}
	return nil
}

// fetchPageWithSize fetches one page of draws from the API with custom page size
func fetchPageWithSize(page int, size int, dateFrom, dateTo int64) ([]Draw, error) {
	url := fmt.Sprintf(
		"%s?game-names=Cash+5&status=CLOSED&size=%d&page=%d&date-from=%d&date-to=%d",
		baseURL,
		size,
		page,
		dateFrom,
		dateTo,
	)

	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", "https://www.njlottery.com/en-us/drawgames/jerseycash.html")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("primary source returned 404: %s", resp.Status)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, err
	}

	return apiResp.Draws, nil
}

// fetchDrawsByDateRange pages through the API for a specific date-from/date-to range.
// Saves intermediate results via saveCallback after each page.
// Stops when it reaches ~365 draws or runs out of data.
func fetchDrawsByDateRange(from, to time.Time, existing []Draw, saveCallback func([]Draw) error) ([]Draw, error) {
	const pageSize = 365 // Fetch a full year at once
	const maxDraws = 365 // Stop after approximately one year of daily draws

	all := append([]Draw(nil), existing...) // copy existing
	page := 0
	fromMs := from.UnixMilli()
	toMs := to.UnixMilli()
	newDrawsCount := 0

	for {
		var draws []Draw
		var err error

		for attempt := 1; attempt <= 5; attempt++ {
			draws, err = fetchPageWithSize(page, pageSize, fromMs, toMs)
			if err != nil {
				if strings.Contains(err.Error(), "500") {
					fmt.Printf("Server 500 on page %d, retry %d/5...\n", page, attempt)
					time.Sleep(2 * time.Second)
					continue
				}
				return nil, err
			}
			break
		}

		if err != nil {
			// If we already have some draws, return what we have instead of erroring out
			if newDrawsCount > 0 {
				fmt.Printf("Error fetching page %d after retries. Saved %d draws successfully.\n", page, newDrawsCount)
				return all, nil
			}
			return nil, fmt.Errorf("page %d failed after retries: %w", page, err)
		}

		if len(draws) == 0 {
			break
		}

		all = append(all, draws...)
		newDrawsCount += len(draws)
		sort.Slice(all, func(i, j int) bool { return all[i].DrawTime < all[j].DrawTime })

		if saveCallback != nil {
			if err := saveCallback(all); err != nil {
				fmt.Printf("Warning: failed to save after page %d: %v\n", page, err)
			}
		}

		// If we got fewer draws than pageSize, there's no more data - stop paging
		if len(draws) < pageSize {
			break
		}

		// For initial fetch with large page size, if we got close to a full year, stop
		// This avoids trying to fetch page 1 which causes 500 errors
		if page == 0 && len(draws) >= 350 {
			break
		}

		// Check if we've already fetched enough
		if newDrawsCount >= maxDraws {
			fmt.Printf("Reached ~365 draws limit, stopping fetch\n")
			break
		}

		page++
	}

	return all, nil
}

// fetchAllDrawsIncremental fetches draws in year-long chunks
func fetchAllDrawsIncremental(existing []Draw, saveCallback func([]Draw) error) ([]Draw, error) {
	var dateFrom, dateTo time.Time

	if len(existing) == 0 {
		// First run: fetch last year (from 1 year ago to now)
		dateTo = time.Now()
		dateFrom = dateTo.AddDate(-1, 0, 0)
	} else {
		// Subsequent runs: fetch the year BEFORE the oldest draw
		sort.Slice(existing, func(i, j int) bool { return existing[i].DrawTime < existing[j].DrawTime })
		oldest := time.UnixMilli(existing[0].DrawTime)

		// Fetch from (oldest - 1 year) to oldest
		dateTo = oldest.Add(-time.Millisecond)
		dateFrom = dateTo.AddDate(-1, 0, 0)
	}

	beforeCount := len(existing)
	allDraws, err := fetchDrawsByDateRange(dateFrom, dateTo, existing, saveCallback)
	newCount := len(allDraws) - beforeCount

	// Print summary line
	periodStr := fmt.Sprintf("%s → %s", dateFrom.Format("2006-01-02"), dateTo.Format("2006-01-02"))
	fmt.Printf("%-33s  %7d  %12d\n", periodStr, newCount, len(allDraws))

	return allDraws, err
}

// fetchCurrentJackpot fetches the most recent draw (any status) to get the current jackpot
func fetchCurrentJackpot() (int64, error) {
	url := fmt.Sprintf("%s?game-names=Cash+5&size=1&page=0", baseURL)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", "https://www.njlottery.com/en-us/drawgames/jerseycash.html")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return 0, err
	}

	if len(apiResp.Draws) > 0 {
		return apiResp.Draws[0].EstimatedJackpot, nil
	}

	return 0, fmt.Errorf("no draws found")
}

// saveDrawsCallback persists draws to the canonical XDG state path resolved
// by configPath() (defaults to $HOME/.local/state/cash5/draws.json).
func saveDrawsCallback(draws []Draw) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	return enc.Encode(draws)
}

// is404Error reports whether an error originated from an HTTP 404 response.
func is404Error(err error) bool {
	return err != nil && strings.Contains(err.Error(), "404")
}

// tryBackupFetchers attempts each backup source in order, merges any draws found
// into existing, saves, and returns the result. Prints status for each attempt.
func tryBackupFetchers(existing []Draw, dateFrom, dateTo time.Time) []Draw {
	for _, fetcher := range backupFetchers {
		fmt.Printf("  Trying %s...\n", fetcher.Name())
		draws, err := fetcher.FetchRecent(dateFrom, dateTo)
		if err != nil {
			fmt.Printf("  %s failed: %v\n", fetcher.Name(), err)
			continue
		}
		if len(draws) == 0 {
			fmt.Printf("  %s: no new draws found\n", fetcher.Name())
			continue
		}

		// Merge: add draws not already in existing (match by date, since IDs differ
		// between primary and backup sources)
		existingDates := make(map[string]bool)
		for _, d := range existing {
			existingDates[time.UnixMilli(d.DrawTime).Format("2006-01-02")] = true
		}
		added := 0
		for _, d := range draws {
			dateKey := time.UnixMilli(d.DrawTime).Format("2006-01-02")
			if !existingDates[dateKey] {
				existing = append(existing, d)
				added++
			}
		}

		sort.Slice(existing, func(i, j int) bool { return existing[i].DrawTime < existing[j].DrawTime })

		if added > 0 {
			fmt.Printf("  %s: added %d draw(s) via backup\n", fetcher.Name(), added)
			if err := saveDrawsCallback(existing); err != nil {
				fmt.Printf("  Warning: failed to save after backup fetch: %v\n", err)
			}
		} else {
			fmt.Printf("  %s: draws already in cache\n", fetcher.Name())
		}
		return existing
	}

	fmt.Printf("%s\n", color.Red3("All backup sources failed — showing cached data"))
	return existing
}
