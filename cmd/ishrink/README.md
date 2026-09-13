## ishrink
Re-encode a HEIC, JPEG, or JPG image as a small JPEG via macOS `sips`.

### Why?
A phone photo is several megabytes, far more than a chat message or a web form wants. `ishrink` re-encodes one image as JPEG at 10% quality, names the output with today's date, and prints the before/after sizes.

```bash
ishrink IMG_4821.heic
==> Shrinking IMG_4821.heic
FILE    NAME                      SIZE
input   IMG_4821.heic          3,412,908
output  IMG_4821_20260908a.jpg   188,211
```

### Usage

```text
ishrink v1.1.0
Shrink a HEIC, JPEG, or JPG image into a small JPEG via macOS sips
github.com/queone/gkit/tree/main/cmd/ishrink

Usage
  ishrink INPUT  Write INPUT's stem plus today's date as a small .jpg next to it

  INPUT is a .heic, .jpeg, or .jpg file. ishrink refuses to overwrite an
  existing file. sips ships with macOS, so ishrink runs only there.

Options
  -v, --version   Print ishrink v1.1.0 and exit
  -h, -?, --help  Show this help and exit

Examples
  ishrink photo.heic  writes photo_20260908a.jpg
```

`INPUT` is a `.heic`, `.jpeg`, or `.jpg` file, in any letter case. The output is the input's stem plus `_YYYYMMDDa.jpg`, written next to it. `ishrink` refuses to overwrite an existing file.

### Quality
10% JPEG quality is the setting the retired `resize_image.sh` script used. It is meant for sharing, not archiving; the original is left untouched.

### Requirements
`sips` ships with macOS, so `ishrink` runs only on a Mac and needs nothing installed.

### See also
- [`vshrink`](../vshrink/README.md) — the video counterpart, for MP4 files.

### Getting Started
This utility is part of a collection of Go utilities. To compile and install follow the **Getting Started** instructions at the [gkit repo](https://github.com/queone/gkit).
