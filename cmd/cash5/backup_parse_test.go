package main

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

const lottoNumbersFragment = `<html><body>
<div class="draw">
  <div class="resultBox">
    <div><strong>Saturday</strong><br>February 21, 2026</div>
    <ul class="balls multiplier">
      <li class="ball ball">14</li>
      <li class="ball ball">20</li>
      <li class="ball ball">3</li>
      <li class="ball ball">41</li>
      <li class="ball bullseye">7</li>
      <li class="ball xtra-number">3</li>
    </ul>
  </div>
  <div class="resultBoxStats"><p>Jackpot: <strong>$764,968</strong></p></div>
</div>
</body></html>`

// The scraper's rows take the API's shape: five numbers ascending and a
// midnight Eastern stamp on the draw date.
func TestBackupRowsMatchTheAPIShape(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(lottoNumbersFragment))
	if err != nil {
		t.Fatal(err)
	}
	draws, err := parseLottoNumbersDraws(doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(draws) != 1 {
		t.Fatalf("draws = %d, want 1", len(draws))
	}
	d := draws[0]
	if got := strings.Join(d.Results[0].Primary, ","); got != "3,7,14,20,41" {
		t.Errorf("numbers = %s, want 3,7,14,20,41", got)
	}
	if want := time.Date(2026, 2, 21, 0, 0, 0, 0, easternTime()).UnixMilli(); d.DrawTime != want {
		t.Errorf("drawTime = %d (%v), want midnight Eastern", d.DrawTime, time.UnixMilli(d.DrawTime).In(easternTime()))
	}
	if d.ID != "lottonumbers-2026-02-21" || d.EstimatedJackpot != 76496800 {
		t.Errorf("id = %s jackpot = %d", d.ID, d.EstimatedJackpot)
	}
}
