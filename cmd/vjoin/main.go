// main.go

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"

	color "github.com/queone/gkit/internal/color"
)

const (
	programName    = "vjoin"
	programVersion = "0.1.0"
	outputPath     = "merged.mp4"
)

type mediaInfo struct {
	vertical bool
}

type probeResult struct {
	Streams []probeStream `json:"streams"`
}

type probeStream struct {
	CodecType    string            `json:"codec_type"`
	Width        int               `json:"width"`
	Height       int               `json:"height"`
	Tags         map[string]string `json:"tags"`
	SideDataList []struct {
		Rotation float64 `json:"rotation"`
	} `json:"side_data_list"`
}

var (
	outputExists = func(path string) (bool, error) {
		_, err := os.Lstat(path)
		if err == nil {
			return true, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	runFFprobe = func(args []string) ([]byte, error) {
		return exec.Command("ffprobe", args...).Output()
	}
	runFFmpeg = func(args []string, stdout, stderr io.Writer) error {
		cmd := exec.Command("ffmpeg", args...)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		return cmd.Run()
	}
)

func usage() string {
	h := color.Whi10
	return fmt.Sprintf("%s v%s\n"+
		"Join two videos into one normalized MP4 by driving ffmpeg.\n"+
		"\n"+
		"%s\n"+
		"  vjoin concatenates INPUT1 followed by INPUT2. Vertical clips receive a\n"+
		"  blurred background; horizontal and square clips receive black padding.\n"+
		"\n"+
		"%s\n"+
		"  vjoin INPUT1 INPUT2\n"+
		"\n"+
		"%s\n"+
		"  Video is normalized to 1920x1080, square pixels, and 30 fps. Audio is\n"+
		"  resampled to 48000 Hz. Output uses H.264 CRF 18 and AAC at 192k.\n"+
		"\n"+
		"%s\n"+
		"  -v, --version          Print %s v%s and exit\n"+
		"  -h, -?, --help         Show this help message and exit\n"+
		"\n"+
		"%s\n"+
		"  Writes merged.mp4 in the current directory and refuses to overwrite it.\n"+
		"  Each input must contain at least one video and one audio stream.\n"+
		"  Requires ffmpeg and ffprobe on PATH (brew install ffmpeg).\n",
		h(programName), programVersion,
		h("Overview"),
		h("Usage"),
		h("Processing"),
		h("Options"),
		programName, programVersion,
		h("Notes"))
}

func run(args []string, stdout, stderr io.Writer) error {
	for _, arg := range args {
		if arg == "-v" || arg == "--version" {
			fmt.Fprintf(stdout, "%s v%s\n", programName, programVersion)
			return nil
		}
		if arg == "-h" || arg == "-?" || arg == "--help" {
			fmt.Fprint(stdout, usage())
			return nil
		}
	}
	if len(args) == 0 {
		fmt.Fprint(stdout, usage())
		return nil
	}
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return fmt.Errorf("unknown flag %q (see %s --help)", arg, programName)
		}
	}
	if len(args) != 2 {
		return fmt.Errorf("expected INPUT1 INPUT2 (see %s --help)", programName)
	}

	return join(args[0], args[1], stdout, stderr)
}

func join(input1, input2 string, stdout, stderr io.Writer) error {
	exists, err := outputExists(outputPath)
	if err != nil {
		return fmt.Errorf("checking output %s: %w; verify directory permissions", outputPath, err)
	}
	if exists {
		return fmt.Errorf("output %s already exists; move or remove it and retry", outputPath)
	}

	first, err := probe(input1)
	if err != nil {
		return err
	}
	second, err := probe(input2)
	if err != nil {
		return err
	}

	args := ffmpegArgs(input1, input2, first.vertical, second.vertical)
	if err := runFFmpeg(args, stdout, stderr); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return fmt.Errorf("ffmpeg not found on PATH; install it with: brew install ffmpeg")
		}
		return fmt.Errorf("joining %s and %s: ffmpeg failed: %w; review ffmpeg output and verify both inputs are valid", input1, input2, err)
	}

	fmt.Fprintf(stdout, "Created: %s\n", outputPath)
	return nil
}

func probe(path string) (mediaInfo, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "stream=codec_type,width,height:stream_tags=rotate:stream_side_data=rotation",
		"-of", "json",
		path,
	}
	body, err := runFFprobe(args)
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return mediaInfo{}, fmt.Errorf("ffprobe not found on PATH; install it with: brew install ffmpeg")
		}
		return mediaInfo{}, fmt.Errorf("probing input %s: %w; verify the file is readable and valid media", path, err)
	}

	var result probeResult
	if err := json.Unmarshal(body, &result); err != nil {
		return mediaInfo{}, fmt.Errorf("parsing ffprobe output for %s: %w; verify ffprobe is working correctly", path, err)
	}

	hasAudio := false
	var video *probeStream
	for i := range result.Streams {
		stream := &result.Streams[i]
		switch stream.CodecType {
		case "video":
			if video == nil {
				video = stream
			}
		case "audio":
			hasAudio = true
		}
	}
	if video == nil {
		return mediaInfo{}, fmt.Errorf("input %s has no video stream; provide a file containing video and audio", path)
	}
	if !hasAudio {
		return mediaInfo{}, fmt.Errorf("input %s has no audio stream; provide a file containing video and audio", path)
	}
	if video.Width <= 0 || video.Height <= 0 {
		return mediaInfo{}, fmt.Errorf("input %s has invalid video dimensions; verify the file is valid media", path)
	}

	return mediaInfo{vertical: isVertical(video.Width, video.Height, streamRotation(*video))}, nil
}

func streamRotation(stream probeStream) int {
	for _, sideData := range stream.SideDataList {
		if sideData.Rotation != 0 {
			return int(math.Round(sideData.Rotation))
		}
	}
	if raw := stream.Tags["rotate"]; raw != "" {
		rotation, err := strconv.Atoi(raw)
		if err == nil {
			return rotation
		}
	}
	return 0
}

func isVertical(width, height, rotation int) bool {
	rotation %= 360
	if rotation < 0 {
		rotation = -rotation
	}
	if rotation == 90 || rotation == 270 {
		width, height = height, width
	}
	return height > width
}

func ffmpegArgs(input1, input2 string, firstVertical, secondVertical bool) []string {
	graph := strings.Join([]string{
		videoFilter(0, firstVertical),
		videoFilter(1, secondVertical),
		"[0:a]aresample=48000[a0]",
		"[1:a]aresample=48000[a1]",
		"[v0][a0][v1][a1]concat=n=2:v=1:a=1[v][a]",
	}, ";")
	return []string{
		"-hide_banner",
		"-n",
		"-i", input1,
		"-i", input2,
		"-filter_complex", graph,
		"-map", "[v]",
		"-map", "[a]",
		"-c:v", "libx264",
		"-preset", "medium",
		"-crf", "18",
		"-c:a", "aac",
		"-b:a", "192k",
		"-movflags", "+faststart",
		outputPath,
	}
}

func videoFilter(index int, vertical bool) string {
	if vertical {
		return fmt.Sprintf("[%d:v]split=2[fg%d][bg%d];[bg%d]scale=1920:1080:force_original_aspect_ratio=increase,crop=1920:1080,boxblur=20:10[blur%d];[fg%d]scale=1920:1080:force_original_aspect_ratio=decrease[front%d];[blur%d][front%d]overlay=(W-w)/2:(H-h)/2,setsar=1,fps=30[v%d]", index, index, index, index, index, index, index, index, index, index)
	}
	return fmt.Sprintf("[%d:v]scale=1920:1080:force_original_aspect_ratio=decrease,pad=1920:1080:(ow-iw)/2:(oh-ih)/2:black,setsar=1,fps=30[v%d]", index, index)
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
		os.Exit(1)
	}
}
