package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestFFmpegArgsOrientationCombinations(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		firstVertical, secondVertical bool
	}{
		{"horizontal horizontal", false, false},
		{"vertical horizontal", true, false},
		{"horizontal vertical", false, true},
		{"vertical vertical", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := ffmpegArgs("first.mp4", "second.mp4", tc.firstVertical, tc.secondVertical)
			graph := argumentAfter(t, args, "-filter_complex")
			for index, vertical := range []bool{tc.firstVertical, tc.secondVertical} {
				label := string(rune('0' + index))
				blur := "[" + label + ":v]split=2"
				pad := "[" + label + ":v]scale=1920:1080:force_original_aspect_ratio=decrease,pad="
				if strings.Contains(graph, blur) != vertical {
					t.Errorf("input %d blur selection = %v, want %v; graph %q", index, strings.Contains(graph, blur), vertical, graph)
				}
				if strings.Contains(graph, pad) == vertical {
					t.Errorf("input %d pad selection = %v, want %v; graph %q", index, strings.Contains(graph, pad), !vertical, graph)
				}
				if vertical {
					for _, want := range []string{
						"[bg" + label + "]scale=1920:1080:force_original_aspect_ratio=increase,crop=1920:1080",
						"[fg" + label + "]scale=1920:1080:force_original_aspect_ratio=decrease",
					} {
						if !strings.Contains(graph, want) {
							t.Errorf("input %d vertical filter missing %q", index, want)
						}
					}
				}
			}
			for _, want := range []string{
				"[0:a]aresample=48000[a0]",
				"[1:a]aresample=48000[a1]",
				"[v0][a0][v1][a1]concat=n=2:v=1:a=1[v][a]",
			} {
				if !strings.Contains(graph, want) {
					t.Errorf("filter graph missing %q", want)
				}
			}
			if containsArg(args, "-y") {
				t.Error("ffmpeg argv must not contain -y")
			}
			if !containsArg(args, "-n") {
				t.Error("ffmpeg argv must contain -n")
			}
			wantTail := []string{
				"-map", "[v]", "-map", "[a]", "-c:v", "libx264", "-preset", "medium",
				"-crf", "18", "-c:a", "aac", "-b:a", "192k", "-movflags", "+faststart", outputPath,
			}
			if !reflect.DeepEqual(args[len(args)-len(wantTail):], wantTail) {
				t.Errorf("ffmpeg argv tail %v, want %v", args[len(args)-len(wantTail):], wantTail)
			}
			if !reflect.DeepEqual(args[:6], []string{"-hide_banner", "-n", "-i", "first.mp4", "-i", "second.mp4"}) {
				t.Errorf("ffmpeg input argv %v", args[:6])
			}
		})
	}
}

func TestRunJoinsAndPreservesStreams(t *testing.T) {
	restoreFakes(t)
	outputExists = func(string) (bool, error) { return false, nil }
	probeCalls := 0
	runFFprobe = func([]string) ([]byte, error) {
		probeCalls++
		return []byte(`{"streams":[{"codec_type":"video","width":1080,"height":1920},{"codec_type":"audio"}]}`), nil
	}
	var gotArgs []string
	var gotStdout, gotStderr io.Writer
	runFFmpeg = func(args []string, stdout, stderr io.Writer) error {
		gotArgs = args
		gotStdout, gotStderr = stdout, stderr
		return nil
	}

	var stdout, stderr bytes.Buffer
	if err := run([]string{"first.mp4", "second.mp4"}, &stdout, &stderr); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if probeCalls != 2 || gotArgs == nil {
		t.Fatalf("probe calls = %d, ffmpeg args = %v", probeCalls, gotArgs)
	}
	if gotStdout != &stdout || gotStderr != &stderr {
		t.Error("ffmpeg output streams were not preserved")
	}
	if stdout.String() != "Created: merged.mp4\n" {
		t.Errorf("stdout = %q", stdout.String())
	}
}

func TestProbeArgumentsAndStreams(t *testing.T) {
	restoreFakes(t)
	var calls [][]string
	runFFprobe = func(args []string) ([]byte, error) {
		calls = append(calls, append([]string(nil), args...))
		return []byte(`{"streams":[{"codec_type":"video","width":1920,"height":1080},{"codec_type":"audio"}]}`), nil
	}

	for _, path := range []string{"first.mp4", "second.mp4"} {
		info, err := probe(path)
		if err != nil {
			t.Fatalf("probe(%q): %v", path, err)
		}
		if info.vertical {
			t.Errorf("probe(%q) classified landscape as vertical", path)
		}
	}
	wantPrefix := []string{
		"-v", "error",
		"-show_entries", "stream=codec_type,width,height:stream_tags=rotate:stream_side_data=rotation",
		"-of", "json",
	}
	for i, path := range []string{"first.mp4", "second.mp4"} {
		want := append(append([]string(nil), wantPrefix...), path)
		if !reflect.DeepEqual(calls[i], want) {
			t.Errorf("probe argv %v, want %v", calls[i], want)
		}
	}
}

func TestOrientationClassification(t *testing.T) {
	for _, tc := range []struct {
		name          string
		width, height int
		rotation      int
		wantVertical  bool
	}{
		{"portrait", 1080, 1920, 0, true},
		{"landscape", 1920, 1080, 0, false},
		{"square", 1080, 1080, 0, false},
		{"positive 90", 1920, 1080, 90, true},
		{"negative 90", 1920, 1080, -90, true},
		{"positive 270", 1920, 1080, 270, true},
		{"negative 270", 1920, 1080, -270, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isVertical(tc.width, tc.height, tc.rotation); got != tc.wantVertical {
				t.Errorf("isVertical(%d, %d, %d) = %v, want %v", tc.width, tc.height, tc.rotation, got, tc.wantVertical)
			}
		})
	}
}

func TestProbeUsesRotationMetadata(t *testing.T) {
	for _, tc := range []struct {
		name, video string
	}{
		{"tag positive", `{"codec_type":"video","width":1920,"height":1080,"tags":{"rotate":"90"}}`},
		{"tag negative", `{"codec_type":"video","width":1920,"height":1080,"tags":{"rotate":"-90"}}`},
		{"side data positive integer", `{"codec_type":"video","width":1920,"height":1080,"side_data_list":[{"rotation":270}]}`},
		{"side data negative integer", `{"codec_type":"video","width":1920,"height":1080,"side_data_list":[{"rotation":-270}]}`},
		{"side data positive decimal", `{"codec_type":"video","width":1920,"height":1080,"side_data_list":[{"rotation":90.0}]}`},
		{"side data negative decimal", `{"codec_type":"video","width":1920,"height":1080,"side_data_list":[{"rotation":-90.0}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restoreFakes(t)
			body := `{"streams":[` + tc.video + `,{"codec_type":"audio"}]}`
			runFFprobe = func([]string) ([]byte, error) { return []byte(body), nil }
			info, err := probe("rotated.mp4")
			if err != nil {
				t.Fatalf("probe failed: %v", err)
			}
			if !info.vertical {
				t.Error("rotated landscape dimensions were not classified as vertical")
			}
		})
	}
}

func TestProbeRequiresVideoAndAudio(t *testing.T) {
	for _, tc := range []struct {
		name, body, missing string
	}{
		{"video", `{"streams":[{"codec_type":"audio"}]}`, "video"},
		{"audio", `{"streams":[{"codec_type":"video","width":1920,"height":1080}]}`, "audio"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restoreFakes(t)
			runFFprobe = func([]string) ([]byte, error) { return []byte(tc.body), nil }
			_, err := probe("affected.mp4")
			if err == nil || !strings.Contains(err.Error(), "affected.mp4") || !strings.Contains(err.Error(), tc.missing) {
				t.Fatalf("probe error = %v", err)
			}
		})
	}
}

func TestExistingOutputStopsBeforeTools(t *testing.T) {
	restoreFakes(t)
	outputExists = func(string) (bool, error) { return true, nil }
	probeCalls, ffmpegCalls := 0, 0
	runFFprobe = func([]string) ([]byte, error) { probeCalls++; return nil, nil }
	runFFmpeg = func([]string, io.Writer, io.Writer) error { ffmpegCalls++; return nil }

	err := run([]string{"first.mp4", "second.mp4"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "already exists") || !strings.Contains(err.Error(), "move or remove") {
		t.Fatalf("run error = %v", err)
	}
	if probeCalls != 0 || ffmpegCalls != 0 {
		t.Errorf("probe calls = %d, ffmpeg calls = %d", probeCalls, ffmpegCalls)
	}
}

func TestHelpAliases(t *testing.T) {
	for _, args := range [][]string{nil, {"-h"}, {"-?"}, {"--help"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			restoreFakes(t)
			outputExists = func(string) (bool, error) { t.Fatal("output checked"); return false, nil }
			var stdout bytes.Buffer
			if err := run(args, &stdout, io.Discard); err != nil {
				t.Fatalf("run(%v): %v", args, err)
			}
			for _, want := range []string{"vjoin", "v0.1.0", "Overview", "Usage", "Processing", "Options", "Notes", "vjoin INPUT1 INPUT2"} {
				if !strings.Contains(stdout.String(), want) {
					t.Errorf("usage missing %q", want)
				}
			}
		})
	}
}

func TestVersionAliases(t *testing.T) {
	for _, flag := range []string{"-v", "--version"} {
		t.Run(flag, func(t *testing.T) {
			restoreFakes(t)
			outputExists = func(string) (bool, error) { t.Fatal("output checked"); return false, nil }
			var stdout, stderr bytes.Buffer
			if err := run([]string{flag}, &stdout, &stderr); err != nil {
				t.Fatalf("run(%s): %v", flag, err)
			}
			if got, want := stdout.String(), programName+" v"+programVersion+"\n"; got != want {
				t.Errorf("stdout = %q, want %q", got, want)
			}
			if got := stderr.String(); got != "" {
				t.Errorf("stderr = %q, want empty", got)
			}
		})
	}
}

func TestInvalidCLIStopsBeforeTools(t *testing.T) {
	for _, args := range [][]string{{"--bad"}, {"one.mp4"}, {"one.mp4", "two.mp4", "three.mp4"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			restoreFakes(t)
			outputExists = func(string) (bool, error) { t.Fatal("output checked"); return false, nil }
			err := run(args, io.Discard, io.Discard)
			if err == nil || !strings.Contains(err.Error(), "vjoin --help") {
				t.Fatalf("run(%v) error = %v", args, err)
			}
		})
	}
}

func TestToolErrors(t *testing.T) {
	t.Run("ffprobe missing", func(t *testing.T) {
		restoreFakes(t)
		outputExists = func(string) (bool, error) { return false, nil }
		runFFprobe = func([]string) ([]byte, error) { return nil, exec.ErrNotFound }
		err := run([]string{"one.mp4", "two.mp4"}, io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "ffprobe not found") || !strings.Contains(err.Error(), "brew install ffmpeg") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("ffprobe failure", func(t *testing.T) {
		restoreFakes(t)
		outputExists = func(string) (bool, error) { return false, nil }
		runFFprobe = func([]string) ([]byte, error) { return nil, errors.New("bad probe") }
		err := run([]string{"one.mp4", "two.mp4"}, io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "probing input one.mp4") || !strings.Contains(err.Error(), "readable") {
			t.Fatalf("error = %v", err)
		}
	})

	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"ffmpeg missing", exec.ErrNotFound, "ffmpeg not found"},
		{"ffmpeg failure", errors.New("encode failed"), "review ffmpeg output"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restoreFakes(t)
			outputExists = func(string) (bool, error) { return false, nil }
			runFFprobe = func([]string) ([]byte, error) {
				return []byte(`{"streams":[{"codec_type":"video","width":1920,"height":1080},{"codec_type":"audio"}]}`), nil
			}
			runFFmpeg = func([]string, io.Writer, io.Writer) error { return tc.err }
			var stdout bytes.Buffer
			err := run([]string{"one.mp4", "two.mp4"}, &stdout, io.Discard)
			if err == nil || !strings.Contains(err.Error(), tc.want) || stdout.Len() != 0 {
				t.Fatalf("error = %v, stdout = %q", err, stdout.String())
			}
		})
	}
}

func TestDocumentation(t *testing.T) {
	utility := readTestFile(t, "README.md")
	for _, want := range []string{
		"refuses to overwrite",
		"at least one video and one audio stream",
		"rotation metadata",
		"vertical clips",
		"horizontal and square clips",
		"1920x1080",
		"ffmpeg",
		"ffprobe",
	} {
		if !strings.Contains(utility, want) {
			t.Errorf("utility README missing %q", want)
		}
	}

	root := readTestFile(t, "../../README.md")
	drop := strings.Index(root, "[`vdrop`]")
	join := strings.Index(root, "[`vjoin`]")
	keep := strings.Index(root, "[`vkeep`]")
	if drop < 0 || join <= drop || keep <= join {
		t.Errorf("root utility order vdrop=%d vjoin=%d vkeep=%d", drop, join, keep)
	}
}

func TestProgramVersion(t *testing.T) {
	if programVersion != "0.1.0" {
		t.Errorf("programVersion = %q, want 0.1.0", programVersion)
	}
}

func restoreFakes(t *testing.T) {
	t.Helper()
	oldExists, oldProbe, oldFFmpeg := outputExists, runFFprobe, runFFmpeg
	t.Cleanup(func() {
		outputExists, runFFprobe, runFFmpeg = oldExists, oldProbe, oldFFmpeg
	})
}

func argumentAfter(t *testing.T, args []string, flag string) string {
	t.Helper()
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	t.Fatalf("argv missing %s: %v", flag, args)
	return ""
}

func containsArg(args []string, want string) bool {
	return slices.Contains(args, want)
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}
