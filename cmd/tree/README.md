## tree
A lightweight directory tree printing utility.

### Why?
Why yet another tree utility?

- **Availability**: If the official tree utility is not installed or hard to get, you can quickly compile this with Go.
- **Cross-Platform**: Easily compile on Linux, Mac, or Windows.
- **Simplicity**: Covers 99% of typical use cases with a minimal feature set.
- **Learning Opportunity**: A great way to practice coding in Go.


### Getting Started
This utility is part of a collection of Go utilities. To compile and install follow the **Getting Started** instructions at the [gkit repo](https://github.com/queone/gkit).

### Usage

```bash
$ tree -?

tree v1.0.3
Directory tree printer — https://github.com/queone/gkit/blob/main/cmd/tree/README.md
Usage
  tree [options] [directory]

  Options can be specified in any order. The last specified directory will be used if
  multiple directories are provided.

Options
  -f                Show full file paths. Can be placed before or after the dir path.
  -?, --help, -h    Show this help message and exit

Examples
  tree
  tree -f /path/to/directory
  tree /path/to/directory -f
  tree -h
```
