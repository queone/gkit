## vdrop
Remove a section of a video — drop `START..END` and join the remainder — by driving `ffmpeg`.

### Why?
Cutting a section out of a video with raw `ffmpeg` means splitting the source, re-listing the pieces, and concatenating them — or wrestling with a `filter_complex` graph for a frame-accurate result — then hand-naming the output. `vdrop` takes a start and (optionally) an end, validates the range against the source up front, joins the surrounding parts, names the output for you, and prints a before/after summary. The mental model is simple: **`vdrop` removes the part you don't want.**

```bash
vdrop 1:00 8:31 SOURCE1.mp4
FILE    NAME           SIZE         DURATION
input   SOURCE1.mp4    104,857,600  00:42:10
output  SOURCE_1.mp4    86,212,000  00:34:39
```

### Usage

```bash
vdrop START [END] [-a] <input>
```

`START` and `END` mark the section to remove. `END` is optional: omit it (or pass the literal `end`) to remove through to the source end.

### Cheatsheet
`vdrop` and `vkeep` are counterparts — every result is reachable from either, but one form is usually shorter. Pick the row that matches your goal:

```
What you want                     Use this
  Copy the whole file             vkeep 0 FILE
  Keep from beginning to 1:00       vkeep 0 1:00 FILE
  Drop from beginning to 1:00       vdrop 0 1:00 FILE
  Keep from 1:00 to the end       vkeep 1:00 FILE
  Drop from 1:00 to the end       vdrop 1:00 FILE
  Keep only the middle 1:00..8:31 vkeep 1:00 8:31 FILE
  Drop only the middle 1:00..8:31 vdrop 1:00 8:31 FILE
```

`vkeep 0 FILE` copies the whole file. `vdrop` has no whole-file form — dropping `0..end` would remove everything, which `vdrop` refuses.

### Timestamps
- `MM:SS` is the default (e.g. `8:31` = 8 min 31 s).
- A bare integer is whole seconds (e.g. `90` = 1 min 30 s).
- `HH:MM:SS` (e.g. `1:08:31`) is accepted only when the source is longer than one hour.
- `END` may be omitted or given as the literal `end` to mean "to the source end".
- A range that exceeds the source duration errors out, and a drop cannot remove the entire video.

### Output naming
The output goes next to the input. Its name is the input's with `_` inserted before the trailing digits (`SOURCE1.mp4` → `SOURCE_1.mp4`), or `_1` appended when the name has no trailing digit (`clip.mp4` → `clip_1.mp4`). `vdrop` refuses to overwrite an existing output.

### Speed vs. accuracy
By default `vdrop` stream-copies (`-c copy`) the surrounding segments and concatenates them — fast and lossless, but each segment boundary snaps to the nearest preceding keyframe. Pass `-a, --accurate` for a frame-accurate result; this re-encodes (`libx264`/`aac`) through a filter graph and is slower.

### Smooth transition
By default the two surrounding parts meet at a hard cut. Pass `-x, --crossfade` to dissolve across the join instead — the tail of the kept "before" part blends into the head of the kept "after" part. The default length is `0.5`s; set your own with `--crossfade=1` (or `-x=1`).

```bash
vdrop -x 1:00 8:31 SOURCE1.mp4             # dissolve, 0.5s
vdrop --crossfade=1 1:00 8:31 SOURCE1.mp4  # dissolve, 1s
```

A crossfade re-encodes (it cannot stream-copy) and applies only to an **interior** drop — both `START` and `END` must fall inside the source, since an edge trim has no second segment to blend. Because the transition overlaps its duration, the output is that many seconds shorter than the equivalent hard cut.

### Requirements
Requires `ffmpeg` and `ffprobe` on your `PATH`:

```bash
brew install ffmpeg
```

### See also
- [`vkeep`](../vkeep/README.md) — the counterpart that **keeps** a section instead of removing it.

### Getting Started
This utility is part of a collection of Go utilities. To compile and install follow the **Getting Started** instructions at the [gkit repo](https://github.com/queone/gkit).
