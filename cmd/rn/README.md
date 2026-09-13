## rn
`rn` is a simple CLI utility for batch renaming files in the current directory.  
It replaces all occurrences of a specified string in filenames with another string.

### Usage

```text
rn v1.6.0
Rename files in the current directory by replacing a substring
github.com/queone/gkit/tree/main/cmd/rn

Usage
  rn "OLD" "NEW" [-f]  Show every file name where OLD becomes NEW; rename with -f

  An empty NEW ("") removes OLD from the name.

Options
  -f              Rename the files instead of only showing the plan
  -v, --version   Print rn v1.6.0 and exit
  -h, -?, --help  Show this help and exit

Examples
  rn "_draft" ""        Show the files that would be renamed
  rn "_draft" "" -f     Rename them
  rn "temp" "final" -f  Replace one substring with another
```
