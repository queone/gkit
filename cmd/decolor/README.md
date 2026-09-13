## decolor
A utility that removes shell color escape codes from input stream or given file.

For example, given: 

```
$ cat -v sample.yaml
^[[94mid^[[0m: ^[[32m18900dbd-6e79-4b12-a344-986faff5a6cd^[[0m
^[[94mdisplayName^[[0m: ^[[32mServicePrincipalName^[[0m
^[[94mappId^[[0m: ^[[32m1cd71455-2ebf-4a27-a56e-1491d22700db^[[0m
```
it can print above without the color codes by either having the content piped to it, or directly loading the file: 

```
$ cat sample.yaml | decolor
...
$ decolor sample.yaml
```

### Usage

```text
decolor v1.2.0
Strip shell color escape codes from a file or piped text
github.com/queone/gkit/tree/main/cmd/decolor

Usage
  decolor FILE   Print FILE without its color escape codes
  ... | decolor  Print piped text without its color escape codes

Options
  -v, --version   Print decolor v1.2.0 and exit
  -h, -?, --help  Show this help and exit

Examples
  cat file | decolor
  decolor /path/to/file
```
