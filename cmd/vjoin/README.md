## vjoin
Join two videos into one normalized MP4 by driving `ffmpeg`.

### Why?
Joining clips with different orientations and dimensions requires a filter graph that normalizes video and audio before concatenation. `vjoin` applies that fixed workflow while preserving the supplied clip order.

### Usage

```bash
vjoin INPUT1 INPUT2
```

The output is always `merged.mp4` in the current directory. `vjoin` refuses to overwrite an existing output; move or remove it before retrying.

### Processing

- Concatenates `INPUT1` followed by `INPUT2`.
- Requires at least one video and one audio stream in each input.
- Detects display orientation from video dimensions and rotation metadata.
- Places vertical clips over a centered, cropped, blurred background derived from the same clip.
- Centers horizontal and square clips on black padding.
- Normalizes video to 1920x1080, square pixels, and 30 fps.
- Resamples audio to 48000 Hz.
- Encodes video with H.264, preset `medium`, and CRF `18`.
- Encodes audio with AAC at `192k`.
- Enables fast-start metadata for streaming playback.

### Options

- `-v, --version` shows the usage screen and version.
- `-h, -?, --help` shows the usage screen.

### Requirements

Requires `ffmpeg` and `ffprobe` on your `PATH`:

```bash
brew install ffmpeg
```

### Getting Started

This utility is part of a collection of Go utilities. To compile and install, follow the **Getting Started** instructions at the [gkit repo](https://github.com/queone/gkit).
