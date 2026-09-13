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

```text
jy v1.7.0
Convert JSON to YAML and YAML to JSON
github.com/queone/gkit/tree/main/cmd/jy

Usage
  jy [options] [FILE]  Convert FILE, or piped input, to the other format

  Options may come in any order. YAML input prints as JSON and JSON input as YAML.

Options
  -c              Print FILE colorized without converting it
  -d              Strip color from the output
  -v, --version   Print jy v1.7.0 and exit
  -h, -?, --help  Show this help and exit

Examples
  cat file | jy
  jy /path/to/file
  jy /path/to/file -d
  jy file.yaml -c      A colorized copy of the file, not converted
```
