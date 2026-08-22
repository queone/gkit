package main

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/queone/gkit/internal/color"
)

type closestMatch struct {
	drawTime  int64
	date      string
	nums      []int
	matches   int
	daysDelta int
}

// quintile returns which quintile (0-4) a number 1-45 falls in.
// Q0: 1-9, Q1: 10-18, Q2: 19-27, Q3: 28-36, Q4: 37-45
func quintile(n int) int {
	return (n - 1) / 9
}

func displayMatchAnalysis(draws []Draw, n int) error {
	if len(draws) == 0 {
		fmt.Println("No draws found")
		return nil
	}

	// Deduplicate and sort oldest first
	seen := make(map[string]bool)
	var uniqueDraws []Draw
	for _, d := range draws {
		if !seen[d.ID] {
			seen[d.ID] = true
			uniqueDraws = append(uniqueDraws, d)
		}
	}
	sort.Slice(uniqueDraws, func(i, j int) bool {
		return uniqueDraws[i].DrawTime < uniqueDraws[j].DrawTime
	})

	// Extract numbers for all draws upfront
	type drawNums struct {
		draw *Draw
		nums []int
		date string
	}
	var parsed []drawNums
	for i := range uniqueDraws {
		nums, err := extractPrimaryFive(&uniqueDraws[i])
		if err != nil {
			continue
		}
		sort.Ints(nums)
		parsed = append(parsed, drawNums{
			draw: &uniqueDraws[i],
			nums: nums,
			date: narrativeDate(time.UnixMilli(uniqueDraws[i].DrawTime)),
		})
	}

	if len(parsed) == 0 {
		fmt.Println("No valid draws found")
		return nil
	}

	// Window the per-draw display loop to the last n entries (entry 0 is
	// always skipped — no prior draw to compare). Pattern analysis below
	// still aggregates across the full parsed slice.
	displayStart := max(1, len(parsed)-n)

	printTimestamp()
	fmt.Println("=== MATCH ANALYSIS ===")
	if n < len(parsed) {
		fmt.Printf("Analyzing %d drawings (of %d total) for closest historical matches...\n\n", len(parsed)-displayStart, len(parsed))
	} else {
		fmt.Printf("Analyzing %d drawings for closest historical matches...\n\n", len(parsed))
	}

	// Track number frequency across all top matches
	matchNumFreq := make(map[int]int)
	// Track quintile co-occurrence: how often matched numbers fall in same quintile
	quintileMatchSame := 0
	quintileMatchTotal := 0
	// Track recency: both matches and total candidates per bucket
	type recencyData struct {
		matches    int
		candidates int
	}
	recencyBuckets := map[string]*recencyData{
		"0-90 days":   {},
		"91-365 days": {},
		"1-3 years":   {},
		"3+ years":    {},
	}
	// Track match distribution shift: first half vs second half of dataset
	var firstHalfDist, secondHalfDist [6]int
	// Track pair frequency in matches
	matchPairFreq := make(map[string]int)
	// Track individual number frequency in matches (for PMI)
	matchIndivFreq := make(map[int]int)
	totalMatchEntries := 0

	// For each draw, find its top 10 closest previous draws
	for i, current := range parsed {
		if i == 0 {
			continue // no previous draws to compare
		}

		var matches []closestMatch
		currentTime := current.draw.DrawTime

		for j := range i {
			prev := parsed[j]
			mc := countMatches(current.nums, prev.nums)
			daysDelta := int(math.Abs(float64(currentTime-prev.draw.DrawTime)) / (1000 * 60 * 60 * 24))
			matches = append(matches, closestMatch{
				drawTime:  prev.draw.DrawTime,
				date:      prev.date,
				nums:      prev.nums,
				matches:   mc,
				daysDelta: daysDelta,
			})
		}

		// Sort: most matches first, ties broken by closest date
		sort.Slice(matches, func(a, b int) bool {
			if matches[a].matches != matches[b].matches {
				return matches[a].matches > matches[b].matches
			}
			return matches[a].daysDelta < matches[b].daysDelta
		})

		limit := min(len(matches), 10)
		topMatches := matches[:limit]

		// Display this draw's matches — windowed to last n drawings.
		// Pattern accumulation below still runs for every i.
		if i >= displayStart {
			payout := formatWinner(uniqueDraws, current.draw)
			numStr := fmt.Sprintf("%02d-%02d-%02d-%02d-%02d",
				current.nums[0], current.nums[1], current.nums[2], current.nums[3], current.nums[4])
			fmt.Printf("%s  %s  5/5 %s\n", color.Grn3(current.date), color.Grn3(numStr), color.Gra5(payout))

			for _, m := range topMatches {
				mNumStr := fmt.Sprintf("%02d-%02d-%02d-%02d-%02d",
					m.nums[0], m.nums[1], m.nums[2], m.nums[3], m.nums[4])
				fmt.Printf("    %s  %s  %s\n",
					color.Grn3(mNumStr), color.Gra5(m.date),
					color.Gra5(fmt.Sprintf("(%d/5 match, %d days prior)", m.matches, m.daysDelta)))
			}
			if isITerm2() {
				displayCircleImage(current.nums, "    ")
			}
			fmt.Println()
		}

		// Count all candidates per recency bucket (for normalization)
		for j := range i {
			delta := int(math.Abs(float64(currentTime-parsed[j].draw.DrawTime)) / (1000 * 60 * 60 * 24))
			switch {
			case delta <= 90:
				recencyBuckets["0-90 days"].candidates++
			case delta <= 365:
				recencyBuckets["91-365 days"].candidates++
			case delta <= 1095:
				recencyBuckets["1-3 years"].candidates++
			default:
				recencyBuckets["3+ years"].candidates++
			}
		}

		isSecondHalf := i >= len(parsed)/2

		// Accumulate pattern data from top matches
		for _, m := range topMatches {
			totalMatchEntries++
			for _, n := range m.nums {
				matchNumFreq[n]++
				matchIndivFreq[n]++
			}
			// Match distribution shift
			if isSecondHalf {
				secondHalfDist[m.matches]++
			} else {
				firstHalfDist[m.matches]++
			}
			// Recency bucket (matches only)
			switch {
			case m.daysDelta <= 90:
				recencyBuckets["0-90 days"].matches++
			case m.daysDelta <= 365:
				recencyBuckets["91-365 days"].matches++
			case m.daysDelta <= 1095:
				recencyBuckets["1-3 years"].matches++
			default:
				recencyBuckets["3+ years"].matches++
			}
			// Pair frequency
			for pi := range len(m.nums) - 1 {
				for pj := pi + 1; pj < len(m.nums); pj++ {
					pair := fmt.Sprintf("%02d-%02d", m.nums[pi], m.nums[pj])
					matchPairFreq[pair]++
				}
			}
			// Quintile clustering: for each pair of matched numbers, check quintile overlap
			matched := make(map[int]bool)
			for _, cn := range current.nums {
				for _, mn := range m.nums {
					if cn == mn {
						matched[cn] = true
					}
				}
			}
			for n1 := range matched {
				for n2 := range matched {
					if n1 < n2 {
						quintileMatchTotal++
						if quintile(n1) == quintile(n2) {
							quintileMatchSame++
						}
					}
				}
			}
		}
	}

	// === PATTERN ANALYSIS ===
	fmt.Println("=== PATTERN ANALYSIS ===")
	fmt.Println()

	// 1. Number frequency in top matches vs uniform 1-45 baseline
	fmt.Printf("%s:\n", color.Blu7("Number Frequency in Top Matches"))

	totalMatchNums := totalMatchEntries * 5

	// Uniform baseline: each of 45 numbers expects an equal share of slots.
	type adjEntry struct {
		num      int
		observed int
		expected float64
		residual float64
	}
	flatExpected := float64(totalMatchNums) / 45.0
	var adjEntries []adjEntry
	for n := 1; n <= 45; n++ {
		observed := matchNumFreq[n]
		residual := float64(observed) - flatExpected
		adjEntries = append(adjEntries, adjEntry{n, observed, flatExpected, residual})
	}

	var variance float64
	for _, e := range adjEntries {
		variance += e.residual * e.residual
	}
	stddev := 0.0
	if len(adjEntries) > 0 {
		stddev = math.Sqrt(variance / float64(len(adjEntries)))
	}

	var overRep, underRep []numCount
	for _, e := range adjEntries {
		if e.residual > 2*stddev {
			overRep = append(overRep, numCount{e.num, e.observed})
		} else if e.residual < -2*stddev {
			underRep = append(underRep, numCount{e.num, e.observed})
		}
	}
	sort.Slice(overRep, func(i, j int) bool { return overRep[i].count > overRep[j].count })
	sort.Slice(underRep, func(i, j int) bool { return underRep[i].count < underRep[j].count })

	fmt.Printf("  %s: %.1f\n", color.Blu7("Adjusted std dev"), stddev)

	if len(overRep) > 0 {
		fmt.Printf("  %s:\n", color.Blu7("Over-represented (>2σ above adjusted expected)"))
		for _, nc := range overRep {
			var exp float64
			for _, e := range adjEntries {
				if e.num == nc.num {
					exp = e.expected
					break
				}
			}
			fmt.Printf("    %s: %s  %s\n",
				color.Grn3(fmt.Sprintf("%02d", nc.num)),
				color.Grn3(fmt.Sprintf("%d times", nc.count)),
				color.Gra5(fmt.Sprintf("(adjusted expected ~%.0f)", exp)))
		}
	} else {
		fmt.Printf("  %s\n", color.Gra5("No numbers significantly over-represented"))
	}
	if len(underRep) > 0 {
		fmt.Printf("  %s:\n", color.Blu7("Under-represented (>2σ below adjusted expected)"))
		for _, nc := range underRep {
			var exp float64
			for _, e := range adjEntries {
				if e.num == nc.num {
					exp = e.expected
					break
				}
			}
			fmt.Printf("    %s: %s  %s\n",
				color.Grn3(fmt.Sprintf("%02d", nc.num)),
				color.Grn3(fmt.Sprintf("%d times", nc.count)),
				color.Gra5(fmt.Sprintf("(adjusted expected ~%.0f)", exp)))
		}
	} else {
		fmt.Printf("  %s\n", color.Gra5("No numbers significantly under-represented"))
	}

	// 2. Value-range quintile clustering
	fmt.Printf("\n%s:\n", color.Blu7("Value-Range Clustering (Quintile Analysis)"))
	fmt.Printf("  %s\n", color.Gra5("How often do matched numbers fall in the same quintile of the pool range?"))
	fmt.Printf("  %s\n", color.Gra5("Quintiles: 1-9, 10-18, 19-27, 28-36, 37-45"))

	if quintileMatchTotal > 0 {
		sameRate := float64(quintileMatchSame) / float64(quintileMatchTotal) * 100
		// Under uniform random, P(same quintile for a pair) = 5*(9/45)^2 = 5*(1/5)^2 = 1/5 = 20%
		expectedRate := 20.0
		fmt.Printf("    %s: %s  %s\n",
			color.Blu7("Same-quintile pair rate"),
			color.Grn3(fmt.Sprintf("%.1f%%", sameRate)),
			color.Gra5(fmt.Sprintf("(%d/%d pairs)", quintileMatchSame, quintileMatchTotal)))
		fmt.Printf("    %s: %s\n",
			color.Blu7("Expected if random"),
			color.Grn3(fmt.Sprintf("%.1f%%", expectedRate)))
		deviation := sameRate - expectedRate
		if math.Abs(deviation) < 3.0 {
			fmt.Printf("    %s: %s\n", color.Blu7("Assessment"),
				color.Grn3(fmt.Sprintf("Within expected range (%.1f%% deviation)", deviation)))
		} else if deviation > 0 {
			fmt.Printf("    %s: %s\n", color.Blu7("Assessment"),
				color.Gra5(fmt.Sprintf("Matched numbers cluster in same value range (+%.1f%%)", deviation)))
		} else {
			fmt.Printf("    %s: %s\n", color.Blu7("Assessment"),
				color.Gra5(fmt.Sprintf("Matched numbers spread across value ranges (%.1f%%)", deviation)))
		}
	} else {
		fmt.Printf("    %s\n", color.Gra5("Insufficient matched-number pairs for analysis"))
	}

	// 3. Recency weighting (density-normalized)
	fmt.Printf("\n%s:\n", color.Blu7("Recency Weighting (Density-Normalized)"))
	fmt.Printf("  %s\n", color.Gra5("Match density = matches per candidate draw in each time window"))
	bucketOrder := []string{"0-90 days", "91-365 days", "1-3 years", "3+ years"}
	for _, bucket := range bucketOrder {
		rd := recencyBuckets[bucket]
		density := 0.0
		if rd.candidates > 0 {
			density = float64(rd.matches) / float64(rd.candidates) * 1000 // per 1000 candidates
		}
		fmt.Printf("    %s: %s  %s  %s\n",
			color.Blu7(fmt.Sprintf("%-13s", bucket)),
			color.Grn3(fmt.Sprintf("%d matches", rd.matches)),
			color.Gra5(fmt.Sprintf("/ %d candidates", rd.candidates)),
			color.Grn3(fmt.Sprintf("(%.2f per 1k)", density)))
	}

	// 4. Match distribution shift over time
	fmt.Printf("\n%s:\n", color.Blu7("Match Distribution Shift Over Time"))
	fmt.Printf("  %s\n", color.Gra5("How top-10 match overlap changes as the candidate pool grows"))
	fmt.Printf("  %s\n", color.Gra5("(first half of dataset vs second half)"))

	firstTotal := 0
	secondTotal := 0
	for k := range 6 {
		firstTotal += firstHalfDist[k]
		secondTotal += secondHalfDist[k]
	}
	fmt.Printf("\n    %-5s  %12s  %12s\n", "MATCH", "FIRST HALF", "SECOND HALF")
	for k := range 6 {
		fp := 0.0
		sp := 0.0
		if firstTotal > 0 {
			fp = float64(firstHalfDist[k]) / float64(firstTotal) * 100
		}
		if secondTotal > 0 {
			sp = float64(secondHalfDist[k]) / float64(secondTotal) * 100
		}
		fmt.Printf("    %d/5    %s  %s\n",
			k,
			color.Grn3(fmt.Sprintf("%5d (%5.1f%%)", firstHalfDist[k], fp)),
			color.Grn3(fmt.Sprintf("%5d (%5.1f%%)", secondHalfDist[k], sp)))
	}
	if firstTotal > 0 && secondTotal > 0 {
		firstAvg := 0.0
		secondAvg := 0.0
		for k := range 6 {
			firstAvg += float64(k) * float64(firstHalfDist[k])
			secondAvg += float64(k) * float64(secondHalfDist[k])
		}
		firstAvg /= float64(firstTotal)
		secondAvg /= float64(secondTotal)
		fmt.Printf("    %s: %.3f → %.3f\n",
			color.Blu7("Avg match count"),
			firstAvg, secondAvg)
		if secondAvg > firstAvg+0.05 {
			fmt.Printf("    %s\n", color.Gra5("Higher overlap in second half — larger pool yields closer matches"))
		} else if firstAvg > secondAvg+0.05 {
			fmt.Printf("    %s\n", color.Gra5("Lower overlap in second half — possible pool diversification"))
		} else {
			fmt.Printf("    %s\n", color.Gra5("Match overlap stable across dataset"))
		}
	}

	// 5. Top pairs with lift score (PMI)
	fmt.Printf("\n%s:\n", color.Blu7("Top Pairs in Closest Matches (Lift-Adjusted)"))
	fmt.Printf("  %s\n", color.Gra5("Lift = observed co-occurrence / expected from individual frequencies"))

	type pairLift struct {
		pair     string
		count    int
		lift     float64
		expected float64
	}
	var pairLifts []pairLift
	if totalMatchEntries > 0 {
		// Under a uniform 1-45 baseline each number has equal share 1/45.
		pairSlots := float64(totalMatchEntries) * 10 // C(5,2) = 10 pair slots per entry
		const pUniform = 1.0 / 45.0
		expectedCount := pairSlots * pUniform * pUniform
		for pair, count := range matchPairFreq {
			lift := 0.0
			if expectedCount > 0 {
				lift = float64(count) / expectedCount
			}
			pairLifts = append(pairLifts, pairLift{pair, count, lift, expectedCount})
		}
	}

	// Sort by lift descending, filter to pairs with meaningful count
	sort.Slice(pairLifts, func(i, j int) bool {
		return pairLifts[i].lift > pairLifts[j].lift
	})

	shown := 0
	for _, pl := range pairLifts {
		if pl.count < 5 {
			continue // skip low-count noise
		}
		fmt.Printf("    %s: %s  %s  %s\n",
			color.Grn3(pl.pair),
			color.Grn3(fmt.Sprintf("%d times", pl.count)),
			color.Gra5(fmt.Sprintf("(expected %.1f)", pl.expected)),
			color.Grn3(fmt.Sprintf("lift %.2fx", pl.lift)))
		shown++
		if shown >= 10 {
			break
		}
	}
	if shown == 0 {
		fmt.Printf("    %s\n", color.Gra5("Insufficient pair data for lift analysis"))
	}

	if len(parsed) < 50 {
		fmt.Printf("\n%s\n", color.Gra5("Note: fewer than 50 draws — pattern analysis may be unreliable"))
	}

	return nil
}
