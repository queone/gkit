## fr
The `fr` utility 3 optional arguments and walks the directory you run it from, examines every regular file that `file --mime-type` reports as a text type, prints each matching line, and optionally replaces the pattern.

If the command line ends with `-f` (i.e. `fr FROM TO -f`) the program writes the replacements.

If `-f` is absent (i.e. `fr FROM TO`) the program does not modify any file – it only prints every line that contains a match, colouring the matched fragment in red and the file name in yellow.

### Search/Replace Types

1. Single-argument search: `fr 'foo.*bar'`

Prints all matching lines with matches highlighted in red. Note that the user can supply any valid regex.

2. Show-only mode: `fr 'FROM' 'TO'`

Highlight occurrences without writing changes.

3. Replace-and-write mode: `fr 'FROM' 'TO' -f`

Replace occurrences in all text files.

### Usage

```text
fr v1.1.0
Find a regular expression in the text files under the current directory, or replace it
github.com/queone/gkit/tree/main/cmd/fr

Usage
  fr REGEX       Print every matching line with its file and line number
  fr FROM TO     Print the lines that FROM matches without changing any file
  fr FROM TO -f  Replace FROM with TO in every matching file

  Hidden directories are skipped. Only files the file command reports as text are read.

Options
  -f              Write the replacement instead of showing matches
  -v, --version   Print fr v1.1.0 and exit
  -h, -?, --help  Show this help and exit
```
