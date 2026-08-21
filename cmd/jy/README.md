## jy
A lightweight JSON and YAML converter utility.


### Why?
Why another YAML / JSON converter utility?

- **Availability**: If the official tree utility is not installed or hard to get, you can quickly compile this with Go.
- **Cross-Platform**: Easily compile on Linux, Mac, or Windows.
- **Simplicity**: Covers 99% of typical use cases with a minimal feature set.
- **Learning Opportunity**: A great way to practice coding in Go.

### Getting Started
This utility is part of a collection of Go utilities. To compile and install follow the **Getting Started** instructions at the [gkit repo](https://github.com/queone/gkit).

### Usage

```bash
jy v1.6.0
JSON / YAML converter - https://github.com/queone/gkit/blob/main/cmd/jy/README.md
Usage
  jy [options] [file]

  Options can be specified in any order. The file can be piped into the utility, or it
  can be referenced as an argument. If the file is YAML, the output will be JSON, or
  vice versa.

Options
  -c                     Colorize the output for the specified file.
  -d                     Decolorize the output for piped input or file.
  -v, --version          Print version and exit.
  -?, --help, -h         Show this help message and exit.

Examples
  cat file | jy
  jy /path/to/file
  jy /path/to/file -d
  jy file.yaml -c        Prints a colorized version of the file. Does not convert.
  jy -h
```
