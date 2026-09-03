package main

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// synthDraw builds a Draw with the given drawTime and primary numbers.
func synthDraw(id string, drawTime int64, nums [5]int) Draw {
	primary := make([]string, 5)
	for i, n := range nums {
		primary[i] = fmt.Sprintf("%d", n)
	}
	return Draw{
		ID:       id,
		DrawTime: drawTime,
		Results:  []Result{{Primary: primary}},
	}
}

func sortedKey(nums []int) [5]int {
	sorted := append([]int(nil), nums...)
	sort.Ints(sorted)
	var key [5]int
	copy(key[:], sorted)
	return key
}

func TestBuildWinnersSet(t *testing.T) {
	draws := []Draw{
		synthDraw("d1", cash5EraStartMillis, [5]int{4, 20, 24, 26, 43}),
		synthDraw("d2", cash5EraStartMillis+86_400_000, [5]int{1, 2, 3, 4, 5}),
	}
	winners := buildWinnersSet(draws)
	if len(winners) != 2 {
		t.Fatalf("len(winners) = %d, want 2", len(winners))
	}
	if !winners[[5]int{4, 20, 24, 26, 43}] {
		t.Error("expected {4,20,24,26,43} in winners")
	}
	if !winners[[5]int{1, 2, 3, 4, 5}] {
		t.Error("expected {1,2,3,4,5} in winners")
	}
}

func TestGenerateRecommendationsAvoidsHistoricalWinners(t *testing.T) {
	// Synthesize a small but varied history so the strategy producers have
	// meaningful rankings. Use 60 draws across distinct combinations.
	var draws []Draw
	base := cash5EraStartMillis
	for i := range 60 {
		combo := [5]int{
			1 + (i*1)%45,
			1 + (i*2+7)%45,
			1 + (i*3+13)%45,
			1 + (i*5+19)%45,
			1 + (i*7+29)%45,
		}
		seen := make(map[int]bool)
		for j := range 5 {
			for seen[combo[j]] {
				combo[j] = combo[j]%45 + 1
			}
			seen[combo[j]] = true
		}
		draws = append(draws, synthDraw(fmt.Sprintf("d%d", i), base+int64(i)*86_400_000, combo))
	}

	winners := buildWinnersSet(draws)
	recs := generateRecommendations(draws, winners)

	wantStrategies := []string{
		"Most common by position",
		"Most frequent",
		"Hot numbers last 30 days",
		"Least common by position",
	}
	if len(recs) != len(wantStrategies) {
		t.Fatalf("len(recs) = %d, want %d", len(recs), len(wantStrategies))
	}
	for i, r := range recs {
		if r.strategy != wantStrategies[i] {
			t.Errorf("recs[%d].strategy = %q, want %q", i, r.strategy, wantStrategies[i])
		}
		key := sortedKey(r.numbers)
		if winners[key] {
			t.Errorf("recommendation %v (strategy %q) is a historical winner",
				r.numbers, r.strategy)
		}
		// Validate range.
		for _, n := range r.numbers {
			if n < 1 || n > 45 {
				t.Errorf("recommendation %v contains out-of-range number %d", r.numbers, n)
			}
		}
		// Validate uniqueness within the combo.
		seen := make(map[int]bool)
		for _, n := range r.numbers {
			if seen[n] {
				t.Errorf("recommendation %v has duplicate number %d", r.numbers, n)
			}
			seen[n] = true
		}
	}
}

func TestFirstUnwonFromTopKSwapsOnCollision(t *testing.T) {
	ranks := []numCount{
		{num: 5, count: 100},
		{num: 10, count: 90},
		{num: 15, count: 80},
		{num: 20, count: 70},
		{num: 25, count: 60},
		{num: 30, count: 50}, // first alternative
		{num: 35, count: 40},
		{num: 40, count: 30},
		{num: 1, count: 20},
		{num: 2, count: 10},
	}
	// Poison the natural top-5 combo so the swap path is exercised.
	winners := map[[5]int]bool{
		{5, 10, 15, 20, 25}: true,
	}
	combo := firstUnwonFromTopK(ranks, winners, 50)
	if combo == nil {
		t.Fatal("expected combo, got nil")
	}
	key := sortedKey(combo)
	if winners[key] {
		t.Errorf("combo %v is the poisoned winner", combo)
	}
	if key == ([5]int{5, 10, 15, 20, 25}) {
		t.Errorf("perturbation did not swap; got %v", combo)
	}
}

func TestFirstUnwonByPositionSwapSwapsOnCollision(t *testing.T) {
	perPos := [5][]numCount{
		{{num: 1, count: 10}, {num: 6, count: 5}},
		{{num: 12, count: 10}, {num: 14, count: 5}},
		{{num: 20, count: 10}, {num: 22, count: 5}},
		{{num: 30, count: 10}, {num: 31, count: 5}},
		{{num: 40, count: 10}, {num: 41, count: 5}},
	}
	winners := map[[5]int]bool{
		{1, 12, 20, 30, 40}: true,
	}
	combo := firstUnwonByPositionSwap(perPos, winners, 50)
	if combo == nil {
		t.Fatal("expected combo, got nil")
	}
	key := sortedKey(combo)
	if winners[key] {
		t.Errorf("combo %v is the poisoned winner", combo)
	}
	if key == ([5]int{1, 12, 20, 30, 40}) {
		t.Errorf("perturbation did not swap; got %v", combo)
	}
}

func TestNextLexComboIndicesAdvances(t *testing.T) {
	idx := []int{0, 1, 2, 3, 4}
	if !nextLexComboIndices(idx, 10) {
		t.Fatal("expected advance from (0,1,2,3,4) over K=10")
	}
	want := []int{0, 1, 2, 3, 5}
	for i := range idx {
		if idx[i] != want[i] {
			t.Fatalf("idx = %v, want %v", idx, want)
		}
	}
}

func TestNextLexComboIndicesReturnsFalseAtEnd(t *testing.T) {
	idx := []int{5, 6, 7, 8, 9}
	if nextLexComboIndices(idx, 10) {
		t.Error("expected false at last combination (5,6,7,8,9)/K=10")
	}
}

func TestRecommendationPreamblePrinted(t *testing.T) {
	out := captureStdout(t, func() {
		fmt.Printf("    %s\n", recommendationPreamble)
	})
	if !strings.Contains(out, "(none of these has previously won)") {
		t.Errorf("preamble missing: %q", out)
	}
}
