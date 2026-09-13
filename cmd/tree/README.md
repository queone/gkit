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

```text
tree v1.1.0
Print a directory tree, with each file's full path on request
github.com/queone/gkit/tree/main/cmd/tree

Usage
  tree [-f] [DIRECTORY]  Print the tree under DIRECTORY, default the current directory

  The flag and the directory may come in any order; the last directory given wins.

Options
  -f              Show each file's full path beside its name
  -v, --version   Print tree v1.1.0 and exit
  -h, -?, --help  Show this help and exit

Examples
  tree
  tree -f /path/to/directory
  tree /path/to/directory -f
```
