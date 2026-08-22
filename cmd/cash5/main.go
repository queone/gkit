package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/queone/gkit/internal/color"

	"github.com/spf13/cobra"
)

const (
	programName     = "cash5"
	programVersion  = "0.13.0"
	lottery_warning = "This is basically lighting money on fire! Play for fun, not profit 😀"
)

// narrativeDate formats a time as "2026-feb-17" for summary/narrative lines
func narrativeDate(t time.Time) string {
	return fmt.Sprintf("%d-%s-%02d", t.Year(), strings.ToLower(t.Format("Jan")), t.Day())
}

func runDailyWithRand() error {
	existing, err := loadDraws()
	if err != nil {
		return err
	}

	// Check connectivity once, up front. All fetch decisions key off this.
	online := checkInternet()
	if !online {
		fmt.Printf("%s\n", color.Red3("Internet is unreachable — showing cached data"))
	}

	// Auto-fetch if no data or data is too old (more than 7 days)
	needsFetch := false
	if len(existing) == 0 {
		if online {
			fmt.Println("Empty local draws.json file. Fetching last 365 drawings...")
			needsFetch = true
		}
	} else {
		// Check if newest draw is more than 7 days old
		sort.Slice(existing, func(i, j int) bool { return existing[i].DrawTime < existing[j].DrawTime })
		newest := time.UnixMilli(existing[len(existing)-1].DrawTime)
		weekAgo := time.Now().AddDate(0, 0, -7)

		if newest.Before(weekAgo) && online {
			fmt.Printf("Data is outdated (newest draw: %s). Fetching recent data...\n",
				narrativeDate(newest))
			needsFetch = true
		}
	}

	if needsFetch {
		allDraws, err := fetchAllDrawsIncremental(existing, saveDrawsCallback)
		if err != nil {
			return fmt.Errorf("failed to fetch draws: %w", err)
		}
		existing = allDraws
		fmt.Println()
	}

	// Fetch all missing recent draws up to yesterday (only when online).
	// Comparison is done in calendar days under the local timezone so the
	// trigger doesn't fire spuriously when the stored drawTime is at UTC
	// midnight and the operator is in a TZ west of UTC.
	if online && len(existing) > 0 {
		sort.Slice(existing, func(i, j int) bool { return existing[i].DrawTime < existing[j].DrawTime })
		newestDrawTime := existing[len(existing)-1].DrawTime
		now := time.Now()
		needs, newest, yesterday := needsRecentFetch(newestDrawTime, now)
		if needs {
			fmt.Printf("Missing recent draws (newest: %s, need up to: %s). Fetching...\n",
				narrativeDate(newest), narrativeDate(yesterday))

			dateFrom := newest.AddDate(0, 0, 1)
			dateTo := now

			recentDraws, err := fetchDrawsByDateRange(dateFrom, dateTo, existing, saveDrawsCallback)
			if err == nil {
				existing = recentDraws
				fmt.Printf("Fetched recent draws. Total in database: %d\n\n", len(existing))
			} else if is404Error(err) {
				// Primary returned 404 — try backup sources transparently
				fmt.Printf("%s\n", color.Red3(fmt.Sprintf("Primary source unavailable (%v) — trying backup...", err)))
				existing = tryBackupFetchers(existing, dateFrom, dateTo)
			} else {
				fmt.Printf("Warning: failed to fetch recent draws: %v\n", err)
			}
		}
	}

	// Display last 10 draws
	if err := displayLastNDraws(existing, 10); err != nil {
		return err
	}

	// Deduplicate and sort for analysis
	sort.Slice(existing, func(i, j int) bool { return existing[i].DrawTime < existing[j].DrawTime })
	seen := make(map[string]bool)
	var uniqueDraws []Draw
	for _, d := range existing {
		if !seen[d.ID] {
			seen[d.ID] = true
			uniqueDraws = append(uniqueDraws, d)
		}
	}

	if len(uniqueDraws) == 0 {
		return fmt.Errorf("no draws available")
	}

	// Build combo -> dates map for repeat checking (narrative format)
	comboHistory := make(map[string][]string)
	for _, d := range uniqueDraws {
		nums, err := extractPrimaryFive(&d)
		if err != nil {
			continue
		}
		sort.Ints(nums)
		key := fmt.Sprintf("%02d-%02d-%02d-%02d-%02d", nums[0], nums[1], nums[2], nums[3], nums[4])
		date := narrativeDate(time.UnixMilli(d.DrawTime))
		comboHistory[key] = append(comboHistory[key], date)
	}

	// Get last winning numbers (LWN)
	lastDraw := uniqueDraws[len(uniqueDraws)-1]
	lwn, err := extractPrimaryFive(&lastDraw)
	if err != nil {
		return fmt.Errorf("failed to extract last winning numbers: %w", err)
	}
	sort.Ints(lwn)
	lwnKey := fmt.Sprintf("%02d-%02d-%02d-%02d-%02d", lwn[0], lwn[1], lwn[2], lwn[3], lwn[4])
	lwnDate := narrativeDate(time.UnixMilli(lastDraw.DrawTime))

	// Current Jackpot (skip fetch if offline)
	if online {
		jackpot, err := fetchCurrentJackpot()
		if err == nil && jackpot > 0 {
			fmt.Printf("  %s: %s\n", color.Blu7("CURRENT JACKPOT"), color.Grn3(formatCurrency(jackpot/100)))
		} else if len(uniqueDraws) > 0 {
			// Fall back to latest draw's estimated jackpot
			jp := uniqueDraws[len(uniqueDraws)-1].EstimatedJackpot
			if jp > 0 {
				fmt.Printf("  %s: %s\n", color.Blu7("CURRENT JACKPOT"), color.Grn3(formatCurrency(jp/100)))
			}
		}
	} else if len(uniqueDraws) > 0 {
		jp := uniqueDraws[len(uniqueDraws)-1].EstimatedJackpot
		if jp > 0 {
			fmt.Printf("  %s: %s %s\n", color.Blu7("CURRENT JACKPOT"),
				color.Grn3(formatCurrency(jp/100)), color.Gra5("(cached)"))
		}
	}

	// LWN repeat check
	fmt.Printf("  %s: %s", color.Blu7("LAST WINNING NUMBERS"), color.Grn3(lwnKey))
	lwnDates := comboHistory[lwnKey]
	if len(lwnDates) > 1 {
		// Filter out the last draw date itself to find prior occurrences
		var priorDates []string
		for _, d := range lwnDates {
			if d != lwnDate {
				priorDates = append(priorDates, d)
			}
		}
		if len(priorDates) > 0 {
			fmt.Printf("  %s", color.Blu7("REPEATED: "+strings.Join(priorDates, ", ")))
		} else {
			fmt.Printf("  %s", color.Gra5("Never repeated"))
		}
	} else {
		fmt.Printf("  %s", color.Gra5("Never repeated"))
	}
	fmt.Println()

	// Winning circle — inline image with winners highlighted (iTerm2 only)
	if isITerm2() {
		fmt.Printf("  %s:\n", color.Blu7("WINNING CIRCLE"))
		displayCircleImage(lwn, "  ")
	}

	// Closest matches to LWN (3+ matching numbers)
	type closeMatch struct {
		drawTime int64
		date     string
		nums     []int
		matches  int
	}
	var closeMatches []closeMatch
	for _, d := range uniqueDraws {
		dt := d.DrawTime
		dDate := narrativeDate(time.UnixMilli(dt))
		if dDate == lwnDate {
			continue
		}
		nums, err := extractPrimaryFive(&d)
		if err != nil {
			continue
		}
		sort.Ints(nums)
		mc := countMatches(lwn, nums)
		if mc >= 3 {
			closeMatches = append(closeMatches, closeMatch{dt, dDate, nums, mc})
		}
	}

	// Sort by match count desc, then by drawTime desc (most recent first within same match count)
	sort.Slice(closeMatches, func(i, j int) bool {
		if closeMatches[i].matches != closeMatches[j].matches {
			return closeMatches[i].matches > closeMatches[j].matches
		}
		return closeMatches[i].drawTime > closeMatches[j].drawTime
	})

	fmt.Printf("  %s:\n", color.Blu7("CLOSEST 5 PREVIOUS WINNING MATCHES"))
	if len(closeMatches) == 0 {
		fmt.Printf("  %s\n", color.Gra5("No previous draws with 3+ matching numbers"))
	} else {
		limit := min(len(closeMatches), 5)
		for _, cm := range closeMatches[:limit] {
			numStr := fmt.Sprintf("%02d-%02d-%02d-%02d-%02d",
				cm.nums[0], cm.nums[1], cm.nums[2], cm.nums[3], cm.nums[4])
			fmt.Printf("    %s  %s  %s\n",
				color.Grn3(numStr), color.Grn3(cm.date),
				color.Gra5(fmt.Sprintf("(%d/5 match)", cm.matches)))
		}
	}

	// Generate intelligent recommendations, guaranteed not to match any
	// historical winning combination.
	winners := buildWinnersSet(uniqueDraws)
	recommendations := generateRecommendations(uniqueDraws, winners)

	fmt.Printf("  %s:\n", color.Blu7("RECOMMENDATION"))
	fmt.Printf("    %s\n", color.Gra5(recommendationPreamble))
	for _, rec := range recommendations {
		numStr := fmt.Sprintf("%02d-%02d-%02d-%02d-%02d",
			rec.numbers[0], rec.numbers[1], rec.numbers[2], rec.numbers[3], rec.numbers[4])
		fmt.Printf("    %s  %s\n", color.Grn3(numStr), color.Gra5(rec.strategy))
	}

	fmt.Printf("\n  %s\n", color.Red3(lottery_warning))

	return nil
}

// recommendationPreamble is the line printed under the RECOMMENDATION header
// asserting that none of the listed combinations has won previously.
const recommendationPreamble = "(none of these has previously won)"

// needsRecentFetch reports whether the newest cached drawTime is at least one
// full calendar day older than `now` in `now`'s local timezone. It returns
// the trigger decision plus the newest and yesterday Time values (in local
// TZ) so the caller can format the user-facing message and bound the fetch
// window. Both stored drawTimes at UTC midnight and the operator's local TZ
// are honored — the check compares date parts, not absolute durations.
func needsRecentFetch(newestDrawTime int64, now time.Time) (bool, time.Time, time.Time) {
	loc := now.Location()
	newest := time.UnixMilli(newestDrawTime).In(loc)
	newestDay := time.Date(newest.Year(), newest.Month(), newest.Day(), 0, 0, 0, 0, loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	yesterday := today.AddDate(0, 0, -1)
	return newestDay.Before(yesterday), newest, yesterday
}

// buildWinnersSet returns a set keyed by the sorted 5-tuple of every draw's
// primary numbers. Draws whose primary cannot be extracted are skipped.
func buildWinnersSet(draws []Draw) map[[5]int]bool {
	winners := make(map[[5]int]bool, len(draws))
	for i := range draws {
		nums, err := extractPrimaryFive(&draws[i])
		if err != nil || len(nums) != 5 {
			continue
		}
		sort.Ints(nums)
		var key [5]int
		copy(key[:], nums)
		winners[key] = true
	}
	return winners
}

type recommendation struct {
	numbers  []int
	strategy string
}

// generateRecommendations creates 5 recommendations based on statistical
// analysis. Every returned combination is absent from the winners set; on
// collision each strategy performs a deterministic single-element swap to the
// next-ranked alternative within its own ranking.
func generateRecommendations(uniqueDraws []Draw, winners map[[5]int]bool) []recommendation {
	// Build frequency maps
	overallFreq := make(map[int]int)
	firstNumFreq := make(map[int]int)
	pos2Freq := make(map[int]int)
	middleNumFreq := make(map[int]int)
	pos4Freq := make(map[int]int)
	lastNumFreq := make(map[int]int)

	// Last 30 days for hot numbers
	latest := time.UnixMilli(uniqueDraws[len(uniqueDraws)-1].DrawTime)
	last30Days := latest.AddDate(0, 0, -30)
	freq30 := make(map[int]int)

	for i := range uniqueDraws {
		nums, err := extractPrimaryFive(&uniqueDraws[i])
		if err == nil && len(nums) == 5 {
			drawDate := time.UnixMilli(uniqueDraws[i].DrawTime)

			firstNumFreq[nums[0]]++
			pos2Freq[nums[1]]++
			middleNumFreq[nums[2]]++
			pos4Freq[nums[3]]++
			lastNumFreq[nums[4]]++

			for _, n := range nums {
				overallFreq[n]++
				if drawDate.After(last30Days) {
					freq30[n]++
				}
			}
		}
	}

	var recs []recommendation

	// 1. Most Common by Position
	perPos := [5][]numCount{
		findTopN(firstNumFreq, 10),
		findTopN(pos2Freq, 10),
		findTopN(middleNumFreq, 10),
		findTopN(pos4Freq, 10),
		findTopN(lastNumFreq, 10),
	}
	if combo := firstUnwonByPositionSwap(perPos, winners, 50); combo != nil {
		recs = append(recs, recommendation{combo, "Most common by position"})
	}

	// 2. Most Frequent Overall (uniform 1-45 baseline → rank by raw count)
	topOverall := findTopN(overallFreq, 10)
	if combo := firstUnwonFromTopK(topOverall, winners, 50); combo != nil {
		recs = append(recs, recommendation{combo, "Most frequent"})
	}

	// 3. Hot Numbers (most frequent in last 30 days)
	topHot := findTopN(freq30, 10)
	if combo := firstUnwonFromTopK(topHot, winners, 50); combo != nil {
		recs = append(recs, recommendation{combo, "Hot numbers last 30 days"})
	}

	// 4. Least Common by Position
	leastPerPos := [5][]numCount{
		findBottomN(firstNumFreq, 10),
		findBottomN(pos2Freq, 10),
		findBottomN(middleNumFreq, 10),
		findBottomN(pos4Freq, 10),
		findBottomN(lastNumFreq, 10),
	}
	if combo := firstUnwonByPositionSwap(leastPerPos, winners, 50); combo != nil {
		recs = append(recs, recommendation{combo, "Least common by position"})
	}

	// 5. Consecutive pair avoidance — the one statistically grounded signal
	consecCombo := generateConsecAvoidComboUnique(winners)
	recs = append(recs, recommendation{consecCombo, "Consecutive pair avoidance"})

	return recs
}

// firstUnwonFromTopK enumerates ascending 5-index subsets of the top-K ranked
// numbers in lexicographic order and returns the first sorted combo that is
// not in winners. Falls back to a random unwon combo (with a stderr warning)
// after maxAttempts or after the rank space is exhausted.
func firstUnwonFromTopK(ranks []numCount, winners map[[5]int]bool, maxAttempts int) []int {
	if len(ranks) < 5 {
		return generateRandomUnwonCombo(winners)
	}
	K := len(ranks)
	idx := []int{0, 1, 2, 3, 4}
	attempts := 0
	for attempts < maxAttempts {
		combo := []int{
			ranks[idx[0]].num, ranks[idx[1]].num, ranks[idx[2]].num,
			ranks[idx[3]].num, ranks[idx[4]].num,
		}
		sort.Ints(combo)
		var key [5]int
		copy(key[:], combo)
		if !winners[key] {
			return combo
		}
		if !nextLexComboIndices(idx, K) {
			break
		}
		attempts++
	}
	fmt.Fprintln(os.Stderr, "cash5: top-K perturbation cap hit; falling back to random unwon combo")
	return generateRandomUnwonCombo(winners)
}

// nextLexComboIndices advances idx (a strictly ascending k-combination over
// [0, K)) to the next combination in lex order. Returns false when idx is the
// last combination.
func nextLexComboIndices(idx []int, K int) bool {
	k := len(idx)
	i := k - 1
	for i >= 0 && idx[i] == K-k+i {
		i--
	}
	if i < 0 {
		return false
	}
	idx[i]++
	for j := i + 1; j < k; j++ {
		idx[j] = idx[j-1] + 1
	}
	return true
}

// firstUnwonByPositionSwap tries the natural pick (rank 0 from each position),
// then deterministically swaps one slot at a time to its next-ranked alternative
// until a sorted combo absent from winners is found or the attempt cap is hit.
// Picks producing duplicate numbers across positions are skipped.
func firstUnwonByPositionSwap(perPos [5][]numCount, winners map[[5]int]bool, maxAttempts int) []int {
	check := func(idx [5]int) []int {
		combo := make([]int, 5)
		seen := make(map[int]bool)
		for p := range 5 {
			if idx[p] >= len(perPos[p]) {
				return nil
			}
			v := perPos[p][idx[p]].num
			if seen[v] {
				return nil
			}
			seen[v] = true
			combo[p] = v
		}
		sort.Ints(combo)
		var key [5]int
		copy(key[:], combo)
		if winners[key] {
			return nil
		}
		return combo
	}

	if r := check([5]int{0, 0, 0, 0, 0}); r != nil {
		return r
	}
	attempts := 0
	for depth := 1; attempts < maxAttempts; depth++ {
		progressed := false
		for slot := range 5 {
			if depth >= len(perPos[slot]) {
				continue
			}
			progressed = true
			attempts++
			idx := [5]int{0, 0, 0, 0, 0}
			idx[slot] = depth
			if r := check(idx); r != nil {
				return r
			}
			if attempts >= maxAttempts {
				break
			}
		}
		if !progressed {
			break
		}
	}
	fmt.Fprintln(os.Stderr, "cash5: position-swap perturbation cap hit; falling back to random unwon combo")
	return generateRandomUnwonCombo(winners)
}

// generateRandomUnwonCombo returns a random 5-number combo absent from winners.
// After a hard cap of 1000 attempts (statistically unreachable) it returns the
// final random combo unconditionally.
func generateRandomUnwonCombo(winners map[[5]int]bool) []int {
	for range 1000 {
		combo := generateRandomCombo()
		var key [5]int
		copy(key[:], combo)
		if !winners[key] {
			return combo
		}
	}
	return generateRandomCombo()
}

// generateConsecAvoidComboUnique is the consec-avoid strategy with a winners
// filter applied: candidates already in winners are rejected outright.
func generateConsecAvoidComboUnique(winners map[[5]int]bool) []int {
	var bestCombo []int
	bestConsec := 0
	for range 1000 {
		combo := generateRandomCombo()
		var key [5]int
		copy(key[:], combo)
		if winners[key] {
			continue
		}
		consec := countConsecPairs(combo)
		if bestCombo == nil || consec < bestConsec {
			bestCombo = combo
			bestConsec = consec
			if bestConsec == 0 {
				break
			}
		}
	}
	if bestCombo == nil {
		fmt.Fprintln(os.Stderr, "cash5: consec-avoid perturbation cap hit; falling back to random unwon combo")
		return generateRandomUnwonCombo(winners)
	}
	return bestCombo
}

func printUsage() {
	n := color.Whi5(programName)
	v := programVersion
	usage := fmt.Sprintf("%s v%s\n"+
		"NJ Cash 5 daily numbers recommender\n"+
		"\n"+
		"%s\n"+
		"  %s [options]\n"+
		"\n"+
		"%s\n"+
		"  -f             Fetch new draws since last run (within last year)\n"+
		"  -a             Display all previous drawings\n"+
		"  -s             Show statistics about historical data\n"+
		"  -m [N]         Show closest-match analysis for last N drawings (default: 30)\n"+
		"  -o [N]         Show odds table for 1 to N combos played (default: 30)\n"+
		"  -d DATE        Show raw JSON for draws on DATE (format: 2026-02-06)\n"+
		"  -v             Show this help message and exit\n"+
		"  -h, -?         Show this help message and exit\n"+
		"\n"+
		"%s\n"+
		"  1. Display the last 10 draws\n"+
		"  2. Show current jackpot, last winning numbers, and closest matches\n"+
		"  3. Recommend 5 sets of numbers based on statistics\n"+
		"\n"+
		"%s\n"+
		"  %s\n"+
		"  %s -f\n"+
		"  %s -s\n"+
		"  %s -m 50\n"+
		"  %s -o 100\n"+
		"  %s -o\n",
		n, v, color.Whi5("Usage"), n, color.Whi5("Options"),
		color.Whi5("Running without switches will"), color.Whi5("Examples"),
		n, n, n, n, n, n)
	usage += "\n" + color.Red3(lottery_warning) + "\n"
	fmt.Print(usage)
}

func runCLI() {
	// Handle -?, -h, --help before cobra — -? isn't a valid pflag character
	for _, arg := range os.Args[1:] {
		if arg == "-?" || arg == "-h" || arg == "--help" {
			printUsage()
			return
		}
	}

	// Handle -o before cobra — cobra can't do optional-value flags properly
	for i, arg := range os.Args[1:] {
		if arg == "-o" {
			n := 30 // default
			if i+1 < len(os.Args[1:]) {
				if val, err := strconv.Atoi(os.Args[i+2]); err == nil && val > 0 {
					n = val
				}
			}
			displayOddsTable(n)
			return
		}
	}

	// Handle -m before cobra — same optional-value reason as -o.
	for i, arg := range os.Args[1:] {
		if arg == "-m" || arg == "--match-analysis" {
			n := 30 // default
			if i+1 < len(os.Args[1:]) {
				if val, err := strconv.Atoi(os.Args[i+2]); err == nil && val > 0 {
					n = val
				}
			}
			existingDraws, err := loadDraws()
			if err != nil {
				log.Fatal(err)
			}
			if err := displayMatchAnalysis(existingDraws, n); err != nil {
				log.Fatal(err)
			}
			return
		}
	}

	var fetchAll bool
	var showVersion bool
	var showAll bool
	var showStats bool
	var debugDate string

	root := &cobra.Command{
		Use:   programName,
		Short: "NJ Cash 5 daily numbers recommender",
		Run: func(cmd *cobra.Command, args []string) {
			if showVersion {
				printUsage()
				return
			}

			if debugDate != "" {
				existingDraws, err := loadDraws()
				if err != nil {
					log.Fatal(err)
				}

				if err := debugDrawByDate(existingDraws, debugDate); err != nil {
					log.Fatal(err)
				}
				return
			}

			if fetchAll {
				fmt.Println("Fetching all historical draws...")
				fmt.Printf("%-33s  %7s  %12s\n", "PERIOD", "DRAWS", "GRAND TOTAL")

				existingDraws, err := loadDraws()
				if err != nil {
					log.Fatal(err)
				}

				for {
					beforeCount := len(existingDraws)

					allDraws, err := fetchAllDrawsIncremental(existingDraws, saveDrawsCallback)
					if err != nil {
						log.Fatal(err)
					}

					newDrawsCount := len(allDraws) - beforeCount
					if newDrawsCount == 0 {
						fmt.Println("\nNo more historical data available.")
						break
					}

					existingDraws = allDraws
				}

				fmt.Printf("\nFetch complete! Total draws in database: %d\n", len(existingDraws))
				return
			}

			if showAll {
				existingDraws, err := loadDraws()
				if err != nil {
					log.Fatal(err)
				}

				if err := displayAllDraws(existingDraws); err != nil {
					log.Fatal(err)
				}
				return
			}

			if showStats {
				existingDraws, err := loadDraws()
				if err != nil {
					log.Fatal(err)
				}

				if err := displayStatistics(existingDraws); err != nil {
					log.Fatal(err)
				}
				return
			}

			if err := runDailyWithRand(); err != nil {
				log.Fatal(err)
			}
		},
	}

	root.Flags().BoolVarP(&fetchAll, "fetch-all", "f", false, "Fetch new draws since last run (within last year)")
	root.Flags().BoolVarP(&showAll, "all", "a", false, "Display all previous drawings")
	root.Flags().BoolVarP(&showStats, "stats", "s", false, "Show statistics about historical data")
	root.Flags().BoolVarP(&showVersion, "version", "v", false, "Show program version and usage")
	root.Flags().StringVarP(&debugDate, "debug", "d", "", "Show raw JSON for draws on specified date")

	// Disable default help flag
	root.SetHelpCommand(&cobra.Command{Hidden: true})
	root.CompletionOptions.DisableDefaultCmd = true
	root.Flags().BoolP("help", "h", false, "")
	root.Flags().Lookup("help").Hidden = true

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main() {
	runCLI()
}
