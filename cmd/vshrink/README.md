## vshrink
Re-encode an MP4 at a high compression level to cut its size, by driving `ffmpeg`.

Audio is copied through ffmpeg's defaults.

### Why?
Phone and screen recordings are far larger than they need to be for sharing. `vshrink` re-encodes one MP4 as H.264 at CRF 35, checks first that the input really is an MP4, names the output with today's date, and prints a before/after summary.

```bash
vshrink clip.mp4
==> Shrinking clip.mp4
FILE    NAME                 SIZE         DURATION
input   clip.mp4             104,857,600  00:03:10
output  clip_20260908a.mp4    21,433,900  00:03:10
```

### Usage

```text
vshrink v1.1.0
Shrink an MP4 by re-encoding it at a high compression level via ffmpeg
github.com/queone/gkit/tree/main/cmd/vshrink

Usage
  vshrink INPUT  Write INPUT's stem plus today's date as a smaller MP4 next to it

  INPUT must be an MP4; vshrink checks the container with ffprobe first. It
  refuses to overwrite an existing file and needs ffmpeg and ffprobe (brew install ffmpeg).

Options
  -v, --version   Print vshrink v1.1.0 and exit
  -h, -?, --help  Show this help and exit

Examples
  vshrink clip.mp4  writes clip_20260908a.mp4
```

`INPUT` must be an MP4; `vshrink` asks `ffprobe` for the container and refuses anything else. The output is the input's stem plus `_YYYYMMDDa.mp4`, written next to it. `vshrink` refuses to overwrite an existing file.

### Quality
CRF 35 favors size over quality. It is the setting the retired `resize_image.sh` script used for video; lower values keep more detail at a larger size.

### Requirements
Requires `ffmpeg` and `ffprobe` on your `PATH`:

```bash
brew install ffmpeg
```

### See also
- [`ishrink`](../ishrink/README.md) — the image counterpart, for HEIC, JPEG, and JPG files.
- [`vconv`](../vconv/README.md) — convert WebM and other formats to MP4.

### Getting Started
This utility is part of a collection of Go utilities. To compile and install follow the **Getting Started** instructions at the [gkit repo](https://github.com/queone/gkit).
