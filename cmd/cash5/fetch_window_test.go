package main

import (
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// apiRow builds an API-style row: numeric id, midnight Eastern on the date.
func apiRow(id, date string, nums ...string) Draw {
	t, _ := time.ParseInLocation("2006-01-02", date, easternTime())
	return Draw{ID: id, GameName: "Cash 5", DrawTime: t.UnixMilli(), Results: []Result{{Primary: nums}}}
}

// backupRow builds a pre-AC101 scraper row: lottonumbers id, 10:57 pm Eastern.
func backupRow(date string, nums ...string) Draw {
	t, _ := time.ParseInLocation("2006-01-02", date, easternTime())
	t = time.Date(t.Year(), t.Month(), t.Day(), 22, 57, 0, 0, easternTime())
	return Draw{ID: "lottonumbers-" + date, GameName: "Cash 5", DrawTime: t.UnixMilli(), Results: []Result{{Primary: nums}}}
}

// tailRows fills every Eastern date from start through end with API rows,
// except the dates in skip, which get no row, and the dates in backup, which
// get a scraper row instead.
func tailRows(start, end string, skip, backup []string) []Draw {
	from, _ := time.ParseInLocation("2006-01-02", start, easternTime())
	to, _ := time.ParseInLocation("2006-01-02", end, easternTime())
	var rows []Draw
	id := 10000
	for day := from; !day.After(to); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		id++
		switch {
		case contains(skip, key):
		case contains(backup, key):
			rows = append(rows, backupRow(key, "4", "18", "29", "36", "15"))
		default:
			rows = append(rows, apiRow(strconv.Itoa(id), key, "1", "2", "3", "4", "5"))
		}
	}
	return rows
}

func contains(list []string, s string) bool {
	return slices.Contains(list, s)
}

var nowSep13 = time.Date(2026, 9, 13, 17, 51, 0, 0, easternTime())

func TestMissingDrawDatesFindsTheHole(t *testing.T) {
	rows := tailRows("2026-07-01", "2026-09-12", []string{"2026-09-08"}, nil)
	got := missingDrawDates(rows, nowSep13, gapLookbackDays)
	if len(got) != 1 || got[0] != "2026-09-08" {
		t.Fatalf("missing = %v, want [2026-09-08]", got)
	}
}

func TestMissingDrawDatesTreatsBackupRowsAsProvisional(t *testing.T) {
	rows := tailRows("2026-07-01", "2026-09-12", []string{"2026-09-08"}, []string{"2026-09-06", "2026-09-07"})
	got := missingDrawDates(rows, nowSep13, gapLookbackDays)
	if strings.Join(got, ",") != "2026-09-06,2026-09-07,2026-09-08" {
		t.Fatalf("missing = %v", got)
	}
}

func TestMissingDrawDatesSkipsChristmas(t *testing.T) {
	now := time.Date(2026, 1, 10, 12, 0, 0, 0, easternTime())
	rows := tailRows("2025-11-01", "2026-01-09", []string{"2025-12-25"}, nil)
	if got := missingDrawDates(rows, now, gapLookbackDays); len(got) != 0 {
		t.Fatalf("missing = %v, want none", got)
	}
}

func TestMissingDrawDatesIsTheSameInAnyZone(t *testing.T) {
	rows := tailRows("2026-07-01", "2026-09-12", []string{"2026-09-08"}, nil)
	for _, name := range []string{"UTC", "America/New_York", "America/Los_Angeles"} {
		loc, err := time.LoadLocation(name)
		if err != nil {
			t.Fatal(err)
		}
		got := missingDrawDates(rows, nowSep13.In(loc), gapLookbackDays)
		if len(got) != 1 || got[0] != "2026-09-08" {
			t.Errorf("%s: missing = %v", name, got)
		}
	}
}

func TestRecentFetchWindowMatchesTheStoreTail(t *testing.T) {
	rows := tailRows("2026-07-01", "2026-09-12", []string{"2026-09-08"}, []string{"2026-09-06", "2026-09-07"})
	from, to, dates, ok := recentFetchWindow(rows, nowSep13)
	if !ok {
		t.Fatal("window not needed")
	}
	if want := time.Date(2026, 9, 5, 23, 0, 0, 0, easternTime()); !from.Equal(want) {
		t.Errorf("from = %v, want %v", from, want)
	}
	if !to.Equal(nowSep13) {
		t.Errorf("to = %v, want now", to)
	}
	if strings.Join(dates, ",") != "2026-09-06,2026-09-07,2026-09-08" {
		t.Errorf("dates = %v", dates)
	}
}

func TestRecentFetchWindowIsQuietWhenComplete(t *testing.T) {
	rows := tailRows("2026-07-01", "2026-09-12", nil, nil)
	if _, _, dates, ok := recentFetchWindow(rows, nowSep13); ok || len(dates) != 0 {
		t.Fatalf("ok = %v dates = %v, want no fetch", ok, dates)
	}
}

func TestRecentFetchWindowStartsAfterAStaleNewestRow(t *testing.T) {
	rows := tailRows("2026-07-01", "2026-09-11", nil, nil)
	from, _, dates, ok := recentFetchWindow(rows, nowSep13)
	if !ok || strings.Join(dates, ",") != "2026-09-12" {
		t.Fatalf("ok = %v dates = %v", ok, dates)
	}
	if want := time.Date(2026, 9, 11, 23, 0, 0, 0, easternTime()); !from.Equal(want) {
		t.Errorf("from = %v, want %v", from, want)
	}
}

func TestMergeDrawsPrefersAPIRows(t *testing.T) {
	api := apiRow("10447", "2026-09-06", "4", "15", "18", "29", "36")
	backup := backupRow("2026-09-06", "4", "18", "29", "36", "15")
	got := mergeDraws([]Draw{backup}, []Draw{api})
	if len(got) != 1 || got[0].ID != "10447" {
		t.Fatalf("api row did not replace backup row: %+v", got)
	}
	got = mergeDraws([]Draw{api}, []Draw{backup})
	if len(got) != 1 || got[0].ID != "10447" {
		t.Fatalf("backup row replaced api row: %+v", got)
	}
	dup := api
	got = mergeDraws(nil, []Draw{api, dup, apiRow("10446", "2026-09-05", "1", "2", "3", "4", "5")})
	if len(got) != 2 || got[0].ID != "10446" || got[1].ID != "10447" {
		t.Fatalf("duplicate not collapsed or unsorted: %+v", got)
	}
}
