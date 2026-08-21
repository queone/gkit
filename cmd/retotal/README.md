## retotal
Financial TOTALS consolidator and re-tallier.

### Why?
Keeping a running financial summary — a budget, an expense sheet — means editing values and recomputing totals by hand. `retotal` keeps an aligned text summary with computed `TOTALS` and re-tallies it in place after you edit it. Every output file is signed with a one-line recalculation note, and re-tally refuses any file whose signature is missing or altered, so a managed file is never silently mis-totalled.

### Usage

```bash
retotal FILE
retotal -h | --help
```

`retotal -h` (or `--help`, or any wrong argument count) prints the information screen and exits 0.

`retotal FILE` picks one of two paths by inspecting `FILE`'s first non-empty line:

**Consolidation** — `FILE` is CSV or space-aligned input. `retotal` computes the summary and writes it to a stem-named text file (`budget.csv` → `budget.txt`) carrying the signature, then prints a hint. It errors without overwriting if the target `.txt` already exists.

CSV input columns: TYPE, DESCRIPTION, MO/AVG, YR/AVG, NOTES

```csv
TYPE,DESCRIPTION,MO/AVG,YR/AVG,NOTES
Income,Salary,5000,60000,primary
Income,Freelance,1500,18000,
,Rent,1200,14400,monthly
```

Output (`budget.txt`):

```
DESCRIPTION            MO/AVG      YR/AVG  NOTES
Income - Salary      5,000.00   60,000.00  primary
Income - Freelance   1,500.00   18,000.00
Rent                 1,200.00   14,400.00  monthly
TOTAL                7,700.00   92,400.00

NOTE: To recalculate TOTALS for this FILE, run `retotal <FILE>`
```

**Re-tally** — `FILE` is already a `retotal` output file (4-column aligned: DESCRIPTION, MO/AVG, YR/AVG, NOTES header). The signature **must** be the last line. `retotal` strips it, normalizes entries, recomputes `TOTAL`, and rewrites in place, re-appending the signature.

```bash
retotal budget.txt
```

If the signature is missing or altered, `retotal` errors immediately — before any computation, leaving the file untouched — and prints the exact line to add:

```
retotal: budget.txt is missing the required signature line; add this as the last line of the file:
NOTE: To recalculate TOTALS for this FILE, run `retotal <FILE>`
```

The `<FILE>` token in the signature is a literal placeholder, so the same signature validates regardless of how the path to `FILE` is given (relative or absolute).

Rows containing "total" in TYPE or DESCRIPTION are skipped from input. All numeric values are normalized to 2 decimal places with thousand separators for values >= 1,000.

### Getting Started
This utility is part of a collection of Go utilities. To compile and install follow the **Getting Started** instructions at the [gkit repo](https://github.com/queone/gkit).
