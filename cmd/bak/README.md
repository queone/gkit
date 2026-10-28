## bak
Create a dated backup copy of a file or directory.

### Why
Makes a dated safety copy before a risky edit in one short command, without remembering `cp` flags or a date format.

### Usage

```text
bak v2.2.0
Create a dated backup copy of a file or directory
github.com/queone/gkit/tree/main/cmd/bak

Usage
  bak PATH  Copy PATH to PATH.YYYYMMDD, adding a letter suffix when that name exists

Options
  -v, --version   Print bak v2.2.0 and exit
  -h, -?, --help  Show this help and exit

Examples
  bak notes.txt  Writes notes.txt.20260912, then notes.txt.20260912a on the next run
  bak project/   Copies the directory tree
```
