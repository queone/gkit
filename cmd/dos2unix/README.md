## dos2unix
Preview or convert CRLF line endings to LF.

### Why
Converts Windows line endings on any machine with Go, even where the classic `dos2unix` is not installed, and can preview the change first.

### Usage

```text
dos2unix v2.2.0
Preview or convert CRLF line endings to LF
github.com/queone/gkit/tree/main/cmd/dos2unix

Usage
  dos2unix FILE     Print FILE with each CRLF shown in blue
  dos2unix FILE -f  Rewrite FILE with LF line endings

Options
  -f              Convert in place instead of previewing
  -v, --version   Print dos2unix v2.2.0 and exit
  -h, -?, --help  Show this help and exit
```
