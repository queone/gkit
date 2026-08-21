## rn
`rn` is a simple CLI utility for batch renaming files in the current directory.  
It replaces all occurrences of a specified string in filenames with another string.

### Usage

```bash
rn v1.5.0
Bulk file re-namer — https://github.com/queone/gkit/blob/main/cmd/rn/README.md

Usage
  rn "OldString" "NewString" [-f]

  Renames all files in the current directory by replacing occurrences of OldString
  in filenames with NewString. If NewString is empty (""), the OldString is removed.

Options
  -f                     Perform actual renaming (required to make changes).
  -?, --help, -h         Show this help message and exit.

Examples
  rn "_draft" ""           Show files that would be renamed (dry run).
  rn "_draft" "" -f       Actually rename files.
  rn "temp" "final" -f     Replace one substring with another.
  rn -h                   Display this help message.
```
