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

```bash
decolor v1.1.1
Text decolorizer - https://github.com/queone/gkit/blob/main/cmd/decolor/README.md
Usage
  decolor [options] [file]

  The file can be piped into the utility, or it can be referenced as an argument.

Options
  |piped input|       Piped text is decolorized
  FILENAME            Decolorize given file path
  -?, --help, -h      Show this help message and exit

Examples
  cat file | decolor
  decolor /path/to/file
  decolor -h
```
