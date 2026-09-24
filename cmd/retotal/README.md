## retotal
Financial TOTALS consolidator and re-tallier.

### Why?
Keeping a running financial summary — a budget, an expense sheet — means editing values and recomputing totals by hand. `retotal` keeps an aligned text summary with computed `TOTALS` and re-tallies it in place after you edit it. Every output file is signed with a one-line recalculation note, and re-tally refuses any file whose signature is missing or altered, or whose columns have drifted out of line, so a managed file is never silently mis-totalled.

### Usage

```text
retotal v1.2.0
Consolidate financial data into a signed TOTALS summary, and re-tally it after edits
github.com/queone/gkit/tree/main/cmd/retotal

Usage
  retotal FILE  Consolidate FILE into <stem>.txt, or re-tally a signed output file in place

  FILE is CSV or space-aligned financial data; the consolidation writes an aligned
  summary with computed TOTALS and a signature line. When FILE is already a signed
  retotal output file, its TOTALS are recomputed in place after you edit it.

Options
  -v, --version   Print retotal v1.2.0 and exit
  -h, -?, --help  Show this help and exit
```

`retotal -h` (or `--help`, or any wrong argument count) prints the information screen and exits 0.

`retotal FILE` picks one of two paths by inspecting `FILE`'s first non-empty line:

**Consolidation** — `FILE` is CSV or space-aligned input. `retotal` computes the summary and writes it to a stem-named text file (`budget.csv` → `budget.txt`) carrying the signature, then prints a hint. It errors without overwriting if the target `.txt` already exists.

CSV input columns: TYPE, DESCRIPTION, MO/AVG, YR/AVG, METHOD, NOTES. METHOD records how each charge is paid, such as `DEBIT` or `PRIME`; it is optional, and any empty or missing METHOD becomes `-` in the output. Header names match in any case.

```csv
TYPE,DESCRIPTION,MO/AVG,YR/AVG,METHOD,NOTES
Income,Salary,5000,60000,,primary
Income,Freelance,1500,18000,,
,Rent,1200,14400,DEBIT,monthly
,Internet,89,1068,PRIME,
```

Output (`budget.txt`):

```
DESCRIPTION           MO/AVG     YR/AVG  METHOD  NOTES
Income - Salary     5,000.00  60,000.00  -       primary
Income - Freelance  1,500.00  18,000.00  -
Rent                1,200.00  14,400.00  DEBIT   monthly
Internet               89.00   1,068.00  PRIME
TOTAL               7,789.00  93,468.00

NOTE: To recalculate above TOTAL line, run `retotal <THIS-FILE>`
```

Space-aligned input matches header names exactly. When its header ends with METHOD then NOTES and a row has a single entry after YR/AVG, that entry is METHOD if it starts left of the NOTES header, and a note otherwise.

**Re-tally** — `FILE` is already a `retotal` output file (aligned, with a DESCRIPTION, MO/AVG, YR/AVG header). The signature **must** be the last line. `retotal` checks that every row lines up, normalizes entries, recomputes `TOTAL`, and rewrites in place, re-appending the signature. An older 4-column file without METHOD gains the column, with `-` on every row.

```bash
retotal budget.txt
```

When the recomputed `TOTAL` differs from the one in the file (compared to the cent), `retotal` prints each changed value; it prints nothing when neither changed. The old value reads `none` when the file had no `TOTAL` row.

```
MO/AVG TOTAL: 7,789.00 -> 7,799.00
YR/AVG TOTAL: 93,468.00 -> 93,588.00
```

Every entry must line up with its header: DESCRIPTION, METHOD, and NOTES entries start in the same column as their header, and MO/AVG and YR/AVG amounts end in the same column as theirs. Columns count characters, so non-ASCII text such as `é` lines up as it looks. Because entries are placed by position, any cell can be left empty, and a note may contain double spaces. If any row has drifted, `retotal` prints the error in red, lists each drifted row, and leaves the file untouched:

```
retotal: budget.txt: column spacing has drifted; line up each entry under its header and rerun:
  line 5: Internet
```

If the signature is missing or altered, `retotal` errors immediately — before any computation, leaving the file untouched — and prints the exact line to add:

```
retotal: budget.txt is missing the required signature line; add this as the last line of the file:
NOTE: To recalculate above TOTAL line, run `retotal <THIS-FILE>`
```

Files signed with the earlier line ``NOTE: To recalculate TOTALS for this FILE, run `retotal <FILE>` `` are still accepted, and re-tally replaces it with the current line.

The `<THIS-FILE>` token in the signature is a literal placeholder, so the same signature validates regardless of how the path to `FILE` is given (relative or absolute).

Rows containing "total" in TYPE or DESCRIPTION are skipped from input. All numeric values are normalized to 2 decimal places with thousand separators for values >= 1,000.

### Getting Started
This utility is part of a collection of Go utilities. To compile and install follow the **Getting Started** instructions at the [gkit repo](https://github.com/queone/gkit).
