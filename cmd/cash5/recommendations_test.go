package main

import (
	"fmt"
	"regexp"
	"slices"
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

// synthHistory builds a small but varied 60-draw history so the strategy
// producers have meaningful rankings.
func synthHistory() []Draw {
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
	return draws
}

// checkValidCombo fails the test when combo is not 5 distinct numbers in 1-45.
func checkValidCombo(t *testing.T, combo []int) {
	t.Helper()
	if len(combo) != 5 {
		t.Errorf("combo %v has %d numbers, want 5", combo, len(combo))
	}
	seen := make(map[int]bool)
	for _, n := range combo {
		if n < 1 || n > 45 {
			t.Errorf("combo %v contains out-of-range number %d", combo, n)
		}
		if seen[n] {
			t.Errorf("combo %v has duplicate number %d", combo, n)
		}
		seen[n] = true
	}
}

// sharedCount counts the numbers two combos have in common.
func sharedCount(a, b []int) int {
	n := 0
	for _, x := range a {
		if slices.Contains(b, x) {
			n++
		}
	}
	return n
}

// exampleSources are the four strategy sets from the design discussion.
var exampleSources = [][]int{
	{1, 10, 19, 33, 43},
	{5, 17, 18, 19, 28},
	{3, 12, 15, 28, 29},
	{3, 7, 8, 30, 40},
}

// exampleAllTime gives the example numbers fixed all-time counts that exercise
// every tie-break: 03 and 28 tie at 40, 01 and 18 tie at 30, and 0-vote 44
// outranks every 1-vote number by count but still ranks after them.
var exampleAllTime = map[int]int{19: 50, 3: 40, 28: 40, 5: 60, 17: 55, 1: 30, 18: 30, 44: 99}

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

func TestGenerateSourceSetsKeepsStrategiesAndAvoidsWinners(t *testing.T) {
	draws := synthHistory()
	winners := buildWinnersSet(draws)
	sources, allTime := generateSourceSets(draws, winners)

	wantStrategies := []string{
		"Most common by position",
		"Most frequent",
		"Hot numbers last 30 days",
		"Least common by position",
	}
	if len(sources) != len(wantStrategies) {
		t.Fatalf("len(sources) = %d, want %d", len(sources), len(wantStrategies))
	}
	for i, s := range sources {
		if s.label != wantStrategies[i] {
			t.Errorf("sources[%d].label = %q, want %q", i, s.label, wantStrategies[i])
		}
		if winners[sortedKey(s.numbers)] {
			t.Errorf("source set %v (strategy %q) is a historical winner", s.numbers, s.label)
		}
		checkValidCombo(t, s.numbers)
	}
	total := 0
	for _, c := range allTime {
		total += c
	}
	if total != 5*len(draws) {
		t.Errorf("all-time counts sum to %d, want %d", total, 5*len(draws))
	}
}

func TestCandidateOrderRanksVotesThenAllTimeThenNumber(t *testing.T) {
	got := candidateOrder(exampleSources, exampleAllTime)
	want := []int{
		19, 3, 28, // 2 votes, ranked by all-time count, tie to the smaller number
		5, 17, 1, 18, 7, 8, 10, 12, 15, 29, 30, 33, 40, 43, // 1 vote
		44, 2, 4, 6, 9, 11, 13, 14, 16, 20, 21, 22, 23, 24, 25, 26, 27,
		31, 32, 34, 35, 36, 37, 38, 39, 41, 42, 45, // 0 votes
	}
	if !slices.Equal(got, want) {
		t.Errorf("candidateOrder =\n  %v\nwant\n  %v", got, want)
	}
}

func TestBlendIsFirstFiveCandidatesSorted(t *testing.T) {
	order := candidateOrder(exampleSources, exampleAllTime)
	blend := slices.Clone(order[:5])
	sort.Ints(blend)
	if want := []int{3, 5, 17, 19, 28}; !slices.Equal(blend, want) {
		t.Errorf("blend = %v, want %v", blend, want)
	}
}

// The example blend 03-05-17-19-28 holds no number above 31, so its two
// weakest numbers, 17 then 05, give way to the best-ranked high numbers, 33
// and 40. The demoted numbers lead the replacement list.
func TestRaiseBlendAddsHighNumbersOnlyWhenNeeded(t *testing.T) {
	raised := raiseBlend(candidateOrder(exampleSources, exampleAllTime))
	if got, want := raised[:5], []int{19, 3, 28, 33, 40}; !slices.Equal(got, want) {
		t.Errorf("raised blend = %v, want %v", got, want)
	}
	if got, want := raised[5:9], []int{5, 17, 1, 18}; !slices.Equal(got, want) {
		t.Errorf("first replacements = %v, want %v", got, want)
	}
	if len(raised) != 45 {
		t.Errorf("len(raised) = %d, want 45", len(raised))
	}

	set := []int{2, 9, 20, 33, 41}
	order := candidateOrder([][]int{set, set, set, set}, nil)
	if got := raiseBlend(order); !slices.Equal(got, order) {
		t.Errorf("blend %v already holds 2 numbers above 31 but changed to %v", set, got[:5])
	}
}

func TestBlendVariationsSwapWeakestFirst(t *testing.T) {
	order := candidateOrder(exampleSources, exampleAllTime)
	recs := blendVariations(order, map[[5]int]bool{})
	want := []recommendation{
		{[]int{3, 19, 28, 33, 43}, "Blend, 40 -> 43"},
		{[]int{3, 19, 28, 40, 44}, "Blend, 33 -> 44"},
		{[]int{3, 5, 19, 33, 40}, "Blend, 28 -> 05"},
		{[]int{17, 19, 28, 33, 40}, "Blend, 03 -> 17"},
	}
	if len(recs) != len(want) {
		t.Fatalf("len(recs) = %d, want %d", len(recs), len(want))
	}
	blend := raiseBlend(order)[:5]
	for i, r := range recs {
		if !slices.Equal(r.numbers, want[i].numbers) || r.label != want[i].label {
			t.Errorf("recs[%d] = %v %q, want %v %q", i, r.numbers, r.label, want[i].numbers, want[i].label)
		}
		if n := sharedCount(r.numbers, blend); n != 4 {
			t.Errorf("recs[%d] %v shares %d numbers with the blend, want 4", i, r.numbers, n)
		}
		if !slices.Contains(r.numbers, blend[0]) {
			t.Errorf("recs[%d] %v lacks the strongest blend number %d", i, r.numbers, blend[0])
		}
		if n := countAboveCeiling(r.numbers); n < minAboveCeiling {
			t.Errorf("recs[%d] %v holds %d numbers above 31, want at least 2", i, r.numbers, n)
		}
		for j := range i {
			if slices.Equal(r.numbers, recs[j].numbers) {
				t.Errorf("recs[%d] and recs[%d] are both %v", j, i, r.numbers)
			}
		}
	}
}

func TestBlendVariationsRetryPastWinner(t *testing.T) {
	order := candidateOrder(exampleSources, exampleAllTime)
	// Poison variation 1's first choice (40 -> 43).
	winners := map[[5]int]bool{{3, 19, 28, 33, 43}: true}
	recs := blendVariations(order, winners)
	wantLabels := []string{
		"Blend, 40 -> 44",
		"Blend, 33 -> 43",
		"Blend, 28 -> 05",
		"Blend, 03 -> 17",
	}
	added := make(map[string]bool)
	for i, r := range recs {
		if r.label != wantLabels[i] {
			t.Errorf("recs[%d].label = %q, want %q", i, r.label, wantLabels[i])
		}
		if winners[sortedKey(r.numbers)] {
			t.Errorf("recs[%d] %v is a past winner", i, r.numbers)
		}
		add := r.label[len(r.label)-2:]
		if added[add] {
			t.Errorf("replacement %s added by more than one variation", add)
		}
		added[add] = true
	}
}

func TestBlendIdenticalSourcesUsesZeroVoteReplacements(t *testing.T) {
	set := []int{2, 9, 20, 33, 41}
	sources := [][]int{set, set, set, set}
	allTime := map[int]int{7: 10, 30: 20, 12: 20}
	order := candidateOrder(sources, allTime)
	blend := slices.Clone(order[:5])
	sort.Ints(blend)
	if !slices.Equal(blend, set) {
		t.Errorf("blend = %v, want %v", blend, set)
	}
	if got, want := order[5:9], []int{12, 30, 7, 1}; !slices.Equal(got, want) {
		t.Errorf("first replacements = %v, want %v", got, want)
	}
}

func TestGenerateRecommendationsReturnsBlendVariations(t *testing.T) {
	draws := synthHistory()
	winners := buildWinnersSet(draws)

	// A source strategy can fall back to a random set on this history, so a
	// second generateSourceSets call may blend differently. Rebuild the blend
	// from each variation (drop NEW, restore OLD) and require all 4 to agree.
	recs := generateRecommendations(draws, winners)
	if len(recs) != 4 {
		t.Fatalf("len(recs) = %d, want 4", len(recs))
	}
	labelRE := regexp.MustCompile(`^Blend, \d{2} -> \d{2}$`)
	var blend []int
	for i, r := range recs {
		if !labelRE.MatchString(r.label) {
			t.Errorf("recs[%d].label = %q, want Blend, NN -> NN", i, r.label)
			continue
		}
		var old, add int
		if _, err := fmt.Sscanf(r.label, "Blend, %d -> %d", &old, &add); err != nil {
			t.Fatalf("parse label %q: %v", r.label, err)
		}
		if slices.Contains(r.numbers, old) {
			t.Errorf("recs[%d] %v: OLD %02d must be missing from the variation", i, r.numbers, old)
		}
		if !slices.Contains(r.numbers, add) {
			t.Errorf("recs[%d] %v: NEW %02d missing from the variation", i, r.numbers, add)
		}
		rebuilt := swapNumber(r.numbers, add, old)
		if blend == nil {
			blend = rebuilt
		} else if !slices.Equal(rebuilt, blend) {
			t.Errorf("recs[%d] %v rebuilds blend %v, want %v shared by every variation", i, r.numbers, rebuilt, blend)
		}
		if winners[sortedKey(r.numbers)] {
			t.Errorf("recs[%d] %v is a historical winner", i, r.numbers)
		}
		if n := countAboveCeiling(r.numbers); n < minAboveCeiling {
			t.Errorf("recs[%d] %v holds %d numbers above 31, want at least 2", i, r.numbers, n)
		}
		checkValidCombo(t, r.numbers)
	}
}

func TestBlendVariationsFallBackToRandomNeverWon(t *testing.T) {
	order := candidateOrder(exampleSources, exampleAllTime)
	// Poison every replacement for variation 1, which drops 40 from the
	// raised blend.
	raised := raiseBlend(order)
	winners := make(map[[5]int]bool)
	for _, add := range raised[5:] {
		winners[comboKey(swapNumber(raised[:5], 40, add))] = true
	}
	var recs []recommendation
	stderr := captureStderr(t, func() {
		recs = blendVariations(order, winners)
	})
	if len(recs) != 4 {
		t.Fatalf("len(recs) = %d, want 4", len(recs))
	}
	if recs[0].label != "Random, never won" {
		t.Errorf("recs[0].label = %q, want %q", recs[0].label, "Random, never won")
	}
	if winners[sortedKey(recs[0].numbers)] {
		t.Errorf("fallback %v is a past winner", recs[0].numbers)
	}
	if n := countAboveCeiling(recs[0].numbers); n < minAboveCeiling {
		t.Errorf("fallback %v holds %d numbers above 31, want at least 2", recs[0].numbers, n)
	}
	checkValidCombo(t, recs[0].numbers)
	if !strings.Contains(stderr, "falling back to random unwon combo") {
		t.Errorf("stderr = %q, want a fallback warning", stderr)
	}
	for i, r := range recs[1:] {
		if !strings.HasPrefix(r.label, "Blend, ") {
			t.Errorf("recs[%d].label = %q, want a Blend swap", i+1, r.label)
		}
	}
}

func TestHelpDefaultDescribesBlendVariations(t *testing.T) {
	want := "3. Blend 4 statistical sets into one and recommend 4 variations of it"
	if !strings.Contains(usage(), want) {
		t.Errorf("help screen lacks %q", want)
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
	if !strings.Contains(out, "(none of these has won since 2020-06-29, and each has at least 2 numbers above 31)") {
		t.Errorf("preamble missing: %q", out)
	}
}
