## cash5
NJ Cash 5 draw history viewer and number recommender.

### Why?
`cash5` keeps a local copy of the Jersey Cash 5 draw history and turns it into a quick daily view: the recent draws, the current jackpot, the closest past matches to the last winning numbers, and 4 recommended sets. The recommendations blend 4 statistical sets by vote and print 4 one-swap variations of the blend. None of them has won since 2020-06-29, and each holds at least 2 numbers above 31, because players favor numbers 1-31 and a jackpot won with those is split more often. They are for fun; every combination has the same 1 in 1,221,759 chance.

`cash5` works only with 1-45 pool draws, from 2020-06-29 onward. The history lives in `$XDG_STATE_HOME/cash5/draws.json` (default `~/.local/state/cash5/draws.json`), and each run fetches any missing draws in that span.

### Getting Started
This utility is part of a collection of Go utilities. To compile and install follow the **Getting Started** instructions at the [gkit repo](https://github.com/queone/gkit).

### Usage

```text
cash5 v0.18.0
Recommend NJ Cash 5 numbers from the draw history
github.com/queone/gkit/tree/main/cmd/cash5

Usage
  cash5 [options]  Show recent draws, the jackpot, closest matches, and recommended sets

Options
  -a              Display all previous drawings
  -s              Show statistics about historical data
  -m [N]          Show closest-match analysis for last N drawings (default: 30)
  -o [N]          Show odds table for 1 to N combos played (default: 30)
  -d DATE         Show raw JSON for draws on DATE (format: 2026-02-06)
  -v, --version   Print cash5 v0.18.0 and exit
  -h, -?, --help  Show this help and exit

Default
  Without options cash5 will
  1. Fetch any missing draws since 2020-06-29, then display the last 10 draws
  2. Show current jackpot, last winning numbers, and closest matches
  3. Blend 4 statistical sets into one and recommend 4 variations of it

  This is basically lighting money on fire! Play for fun, not profit 😀

Examples
  cash5
  cash5 -s
  cash5 -m 50
  cash5 -o 100
  cash5 -o
```

### Observations
Dated findings from the draw history, kept to inform future changes to the recommendations. Add new entries at the top. A "column" is a position after sorting a draw's 5 numbers in ascending order. Every entry uses only 1-45 pool draws, from 2020-06-29 onward, except the entry that dates the pool change.

#### 2026-09-29: Players favor numbers 1-31 (cash5 v0.17.0)
- Data: 2,276 draws from 2020-06-29 to 2026-09-28 that carry prize counts.
- Each draw's 3/5 winner count is divided by the median of the surrounding 31 draws to remove sales swings, then scaled so the average draw is 1.00.
- On a fair draw, a night's winner count depends only on how many tickets held the drawn numbers. More winners means more players picked those numbers.

| Numbers above 31 | Draws | 3/5 winners vs average | Jackpot winners per draw |
|---|---|---|---|
| 0 | 335 | 1.31 | 0.478 |
| 1 | 841 | 1.04 | 0.216 |
| 2 | 728 | 0.91 | 0.207 |
| 3 | 312 | 0.82 | 0.160 |
| 4 | 57 | 0.76 | 0.193 |
| 5 | 3 | 0.77 | 0.000 |

- Draws with 0 or 1 numbers above 31 averaged 0.291 jackpot winners. Draws with 2 or more averaged 0.193.
- Half of all combinations hold at least 2 numbers above 31.
- Implication: a set with at least 2 numbers above 31 has the same odds, but a won jackpot is split less often.
- Decision: adopted in cash5 v0.18.0.

#### 2026-09-29: Frequency strategies do not predict the next draw (cash5 v0.17.0)
- Data: 2,277 draws from 2020-06-29 to 2026-09-28. The prediction test skips the first 365 draws to build history, so it covers 1,912 draws.
- The 45 numbers come up evenly: counts range from 224 to 288 against 253 expected. The chi-square is 41.9 with 44 degrees of freedom, well under the 60.5 cutoff for bias.
- Each next draw held, on average, 1.093 numbers from the all-time top 10, 1.101 from the 30-day hot top 10, and 1.107 from the all-time bottom 10.
- A random group of 10 numbers expects 1.111, with a standard error near 0.020. All three strategies fall within chance.
- Implication: the blend's source sets carry no edge and no penalty. A recommended combination is exactly as likely to win as a random pick.

#### 2026-09-29: The 1-45 pool started on 2020-06-29 (cash5 v0.17.0)
- Numbers 44 and 45 never appear before 2020-06-29. The first 45 came on 2020-06-29 and the first 44 on 2020-07-10.
- The base jackpot rose from $75,000 to $100,000 with the 2020-06-29 draw.
- cash5 treats 2014-09-14 as the start of the 1-45 pool, so 2,109 draws from the 1-43 pool feed its statistics.
- Across the full history, 44 and 45 show 232 and 288 draws, while every other number shows 455 to 550.
- Implication: the all-time and by-position statistics undercount 44 and 45. The odds of a combination are unaffected.
- Decision: cash5 v0.18.0 works only with draws from 2020-06-29 onward.

#### 2026-09-29: Jackpots are often split (cash5 v0.17.0)
- Data: the stored prize data for 1-45 pool draws from 2020-06-29 to 2026-09-28.
- 453 draws list at least one 5/5 winner.
- 87 of those draws split the jackpot: 72 between 2 winners, 13 among 3, and 2 among 4.
- Implication: picking combinations that few other players choose cannot raise the chance of winning. It can raise the share of a won jackpot.
- Decision: cash5 v0.18.0 keeps at least 2 numbers above 31 in each set.

#### 2026-09-29: Next-night repeats match chance (cash5 v0.17.0)
- Question: does a number rarely repeat in the same column the night after it is drawn, so that recommendations should skip the previous night's numbers?
- Data: 2,276 back-to-back draw pairs from 2020-06-29 to 2026-09-28.

| Column | Same number as the night before | Fair-draw expectation |
|---|---|---|
| 1 | 6.24% | 6.46% |
| 2 | 3.65% | 3.70% |
| 3 | 3.30% | 3.33% |
| 4 | 4.31% | 3.70% |
| 5 | 7.43% | 6.46% |

- Any column: 22.5% of draws repeat at least one previous-night number in the same column, against 21.1% for a fair draw.
- Any position: 47.5% of draws share at least one number with the night before, against 46.1% for a fair draw.
- Short windows swing both ways. Same-column repeats totaled 0 in the last 7 draws (1.7 expected), 3 in the last 30 (7.1 expected), and 94 in the last 365 (86.3 expected).
- Finding: repeat rates match a fair draw. If anything, repeats run slightly above chance, within normal variation. A low count over a week or a month is a chance swing, not a pattern.
- Implication: skipping the previous night's numbers leaves the odds of every combination unchanged. It is a preference filter, not an edge.
- Decision: a filter that skips the previous night's numbers was considered and not adopted.
