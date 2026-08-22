// Package vedit is the shared engine behind the vkeep and vdrop utilities: it
// drives ffmpeg/ffprobe to keep or remove a START..END section of a video,
// validates the requested range against the source, names the output, and
// prints a before/after summary. vkeep and vdrop are thin command wrappers over
// Keep and Drop, and share a single help screen rendered by Usage.
package vedit

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	color "github.com/queone/gkit/internal/color"
)

// Injectable seams, overridden in tests to avoid invoking real binaries.
var (
	lookPath = exec.LookPath

	runFFmpeg = func(args []string) error {
		cmd := exec.Command("ffmpeg", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	runFFprobe = func(args []string) ([]byte, error) {
		return exec.Command("ffprobe", args...).Output()
	}

	makeTempDir = func() (string, error) { return os.MkdirTemp("", "vedit-") }
)

// parseOffset parses a timestamp's syntax into whole seconds, independent of any
// source duration. It returns the seconds, the colon-separated component count
// (1, 2, or 3), and an error for malformed input.
func parseOffset(s string) (int, int, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 1 || len(parts) > 3 {
		return 0, 0, fmt.Errorf("invalid timestamp %q: use SS, MM:SS, or HH:MM:SS", s)
	}
	nums := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return 0, 0, fmt.Errorf("invalid timestamp %q: fields must be non-negative integers", s)
		}
		nums[i] = n
	}
	switch len(nums) {
	case 1:
		return nums[0], 1, nil
	case 2:
		if nums[1] > 59 {
			return 0, 0, fmt.Errorf("invalid timestamp %q: seconds must be 00-59", s)
		}
		return nums[0]*60 + nums[1], 2, nil
	default: // 3
		if nums[1] > 59 || nums[2] > 59 {
			return 0, 0, fmt.Errorf("invalid timestamp %q: minutes and seconds must be 00-59", s)
		}
		return nums[0]*3600 + nums[1]*60 + nums[2], 3, nil
	}
}

// validateTimestamp applies the duration-gated rule: a 3-component HH:MM:SS
// timestamp is permitted only when the source is longer than one hour.
func validateTimestamp(components int, durationSeconds float64) error {
	if components == 3 && durationSeconds <= 3600 {
		return errors.New("HH:MM:SS form is only allowed when the source is longer than one hour")
	}
	return nil
}

// parseAndGate parses a timestamp and applies the duration gate, returning seconds.
func parseAndGate(ts string, duration float64) (int, error) {
	secs, comps, err := parseOffset(ts)
	if err != nil {
		return 0, err
	}
	if err := validateTimestamp(comps, duration); err != nil {
		return 0, fmt.Errorf("%q: %w", ts, err)
	}
	return secs, nil
}

// resolveEnd turns an END token into whole seconds: the literal "end" yields the
// source duration (run to the very end), any other token is parsed and gated.
func resolveEnd(token string, duration float64) (int, error) {
	if token == "end" {
		return int(duration), nil
	}
	return parseAndGate(token, duration)
}

// validateClip enforces the keep range 0 <= start < end <= source duration.
func validateClip(start, end int, d float64) error {
	if start < 0 || start >= end || float64(end) > d {
		return fmt.Errorf("require 0 <= start < end <= source duration %s", formatDuration(int(d)))
	}
	return nil
}

// validateCut enforces the keep-range bounds and additionally rejects a cut that
// would remove the entire video (start at 0 reaching the source end).
func validateCut(start, end int, d float64) error {
	if err := validateClip(start, end, d); err != nil {
		return err
	}
	if start == 0 && end >= int(d) {
		return errors.New("cut would remove the entire video")
	}
	return nil
}

// validateCrossfade enforces that a crossfade applies only to an interior drop
// (both boundaries strictly inside the source) and that its duration fits within
// each kept segment, so the dissolve has room to overlap on both sides.
func validateCrossfade(start, end int, d, crossfade float64) error {
	if start == 0 || end >= int(d) {
		return errors.New("crossfade requires an interior drop — START and END must both fall inside the source")
	}
	if crossfade <= 0 {
		return errors.New("crossfade duration must be greater than zero")
	}
	if crossfade > float64(start) {
		return fmt.Errorf("crossfade duration %gs exceeds the %ds kept before the drop", crossfade, start)
	}
	if crossfade > d-float64(end) {
		return fmt.Errorf("crossfade duration %gs exceeds the %ds kept after the drop", crossfade, int(d)-end)
	}
	return nil
}

// deriveOutputName builds the output filename from an input basename: it inserts
// '_' before the trailing run of digits in the stem (SOURCE1.mp4 -> SOURCE_1.mp4),
// or appends '_1' when the stem has no trailing digit (clip.mp4 -> clip_1.mp4).
func deriveOutputName(name string) string {
	ext := filepath.Ext(name)
	stem := name[:len(name)-len(ext)]
	i := len(stem)
	for i > 0 && stem[i-1] >= '0' && stem[i-1] <= '9' {
		i--
	}
	if i == len(stem) {
		return stem + "_1" + ext
	}
	return stem[:i] + "_" + stem[i:] + ext
}

// formatBytes renders a byte count with thousands separators (1234567 -> "1,234,567").
func formatBytes(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var b strings.Builder
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(ch)
	}
	out := b.String()
	if neg {
		out = "-" + out
	}
	return out
}

// formatDuration renders whole seconds as zero-padded HH:MM:SS (4111 -> "01:08:31").
func formatDuration(totalSeconds int) string {
	if totalSeconds < 0 {
		totalSeconds = 0
	}
	h := totalSeconds / 3600
	m := (totalSeconds % 3600) / 60
	s := totalSeconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// probeDurationSeconds returns the total stream duration of path via ffprobe.
func probeDurationSeconds(path string) (float64, error) {
	out, err := runFFprobe([]string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	})
	if err != nil {
		return 0, fmt.Errorf("probing %q: %w (is it a valid video file?)", path, err)
	}
	s := strings.TrimSpace(string(out))
	d, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("probing %q: could not parse duration %q", path, s)
	}
	return d, nil
}

// toolsAvailable verifies ffmpeg and ffprobe are on PATH, with an install hint.
func toolsAvailable() error {
	for _, t := range []string{"ffmpeg", "ffprobe"} {
		if _, err := lookPath(t); err != nil {
			return fmt.Errorf("%s not found on PATH — install it with: brew install ffmpeg", t)
		}
	}
	return nil
}

// Plan is the ordered set of ffmpeg commands for one operation, plus an optional
// concat-list file to materialize before the final command (used by the interior
// copy cut). An empty Plan signals a whole-file byte copy.
type Plan struct {
	Cmds       [][]string // each entry is the argv passed to ffmpeg
	ConcatList string     // path of the concat-list file to write, or "" if none
	ConcatBody string     // contents of that file
}

// keep returns the command plan that keeps start..end. It dispatches on the
// resolved bounds: a whole-file keep is signalled with an empty Plan (byte copy);
// start at 0 trims from the right (-to); end at the source duration trims from the
// left (-ss); otherwise it keeps the middle segment (-ss/-t). Accurate variants
// re-encode (libx264/aac); fast variants stream-copy.
func keep(start, end int, accurate bool, in, out string, d float64) Plan {
	full := end >= int(d)
	switch {
	case start == 0 && full:
		return Plan{} // whole file: caller byte-copies
	case start == 0:
		to := strconv.Itoa(end)
		if accurate {
			return Plan{Cmds: [][]string{{"-i", in, "-to", to, "-c:v", "libx264", "-c:a", "aac", out}}}
		}
		return Plan{Cmds: [][]string{{"-i", in, "-to", to, "-c", "copy", out}}}
	case full:
		ss := strconv.Itoa(start)
		if accurate {
			return Plan{Cmds: [][]string{{"-i", in, "-ss", ss, "-c:v", "libx264", "-c:a", "aac", out}}}
		}
		return Plan{Cmds: [][]string{{"-ss", ss, "-i", in, "-c", "copy", out}}}
	default:
		ss := strconv.Itoa(start)
		dur := strconv.Itoa(end - start)
		if accurate {
			return Plan{Cmds: [][]string{{"-i", in, "-ss", ss, "-t", dur, "-c:v", "libx264", "-c:a", "aac", out}}}
		}
		return Plan{Cmds: [][]string{{"-ss", ss, "-i", in, "-t", dur, "-c", "copy", out}}}
	}
}

// cut returns the command plan that removes start..end and joins the remainder.
// A removal anchored at the start reduces to a left-trim keep; one reaching the
// end reduces to a right-trim keep; an interior removal stream-copies the two
// surrounding segments and concatenates them (fast), or re-encodes through a
// filter graph (accurate). tmpDir holds the intermediate segments for the
// interior copy path.
func cut(start, end int, accurate bool, in, out, tmpDir string, d float64) Plan {
	switch {
	case start == 0:
		return keep(end, int(d), accurate, in, out, d)
	case end >= int(d):
		return keep(0, start, accurate, in, out, d)
	default:
		if accurate {
			return Plan{Cmds: [][]string{{"-i", in, "-filter_complex", cutFilter(start, end), "-map", "[v]", "-map", "[a]", out}}}
		}
		ext := filepath.Ext(in)
		seg1 := filepath.Join(tmpDir, "seg0"+ext)
		seg2 := filepath.Join(tmpDir, "seg1"+ext)
		list := filepath.Join(tmpDir, "concat.txt")
		return Plan{
			Cmds: [][]string{
				{"-i", in, "-to", strconv.Itoa(start), "-c", "copy", seg1},
				{"-ss", strconv.Itoa(end), "-i", in, "-c", "copy", seg2},
				{"-f", "concat", "-safe", "0", "-i", list, "-c", "copy", out},
			},
			ConcatList: list,
			ConcatBody: fmt.Sprintf("file '%s'\nfile '%s'\n", seg1, seg2),
		}
	}
}

// crossfadeFilter builds the -filter_complex graph that keeps [0,start] and
// [end,duration] and dissolves between them over dur seconds, re-encoding for a
// smooth join. The video transition begins dur seconds before the cut
// (offset = start - dur); audio crossfades over the same window.
func crossfadeFilter(start, end int, dur float64) string {
	off := float64(start) - dur
	return fmt.Sprintf(
		"[0:v]trim=0:%d,setpts=PTS-STARTPTS[v0];"+
			"[0:v]trim=start=%d,setpts=PTS-STARTPTS[v1];"+
			"[v0][v1]xfade=transition=fade:duration=%g:offset=%g[v];"+
			"[0:a]atrim=0:%d,asetpts=PTS-STARTPTS[a0];"+
			"[0:a]atrim=start=%d,asetpts=PTS-STARTPTS[a1];"+
			"[a0][a1]acrossfade=d=%g[a]",
		start, end, dur, off, start, end, dur)
}

// cutFilter builds the -filter_complex graph that drops [start,end] and
// concatenates the surrounding parts, re-encoding for a frame-accurate cut.
func cutFilter(start, end int) string {
	return fmt.Sprintf(
		"[0:v]trim=0:%d,setpts=PTS-STARTPTS[v0];"+
			"[0:a]atrim=0:%d,asetpts=PTS-STARTPTS[a0];"+
			"[0:v]trim=start=%d,setpts=PTS-STARTPTS[v1];"+
			"[0:a]atrim=start=%d,asetpts=PTS-STARTPTS[a1];"+
			"[v0][a0][v1][a1]concat=n=2:v=1:a=1[v][a]",
		start, start, end, end)
}

// Keep writes the START..END section of input to the derived output.
// endTok may be a timestamp or the literal "end" (run to the source end).
func Keep(accurate bool, startTok, endTok, input string) error {
	return process(false, accurate, 0, startTok, endTok, input)
}

// Drop removes the START..END section of input and joins the remainder, writing
// the result to the derived output. endTok may be a timestamp or the literal
// "end" (remove through to the source end). A crossfade greater than zero
// dissolves across the interior join over that many seconds (re-encodes;
// interior drops only).
func Drop(accurate bool, crossfade float64, startTok, endTok, input string) error {
	return process(true, accurate, crossfade, startTok, endTok, input)
}

// Usage returns the shared help screen printed by both vkeep and vdrop. invoked
// is the command name the user typed; its rows in the cheatsheet are highlighted
// so each tool points the reader at its own shorter form. The body is identical
// for both tools apart from that highlight and the header line, so the pair is
// documented from a single source.
func Usage(invoked, version string) string {
	h := color.Whi10
	rows := []struct{ goal, cmd string }{
		{"Copy the whole file", "vkeep 0 FILE"},
		{"Keep from beginning to 1:00", "vkeep 0 1:00 FILE"},
		{"Drop from beginning to 1:00", "vdrop 0 1:00 FILE"},
		{"Keep from 1:00 to the end", "vkeep 1:00 FILE"},
		{"Drop from 1:00 to the end", "vdrop 1:00 FILE"},
		{"Keep only the middle 1:00..8:31", "vkeep 1:00 8:31 FILE"},
		{"Drop only the middle 1:00..8:31", "vdrop 1:00 8:31 FILE"},
	}
	var table strings.Builder
	fmt.Fprintf(&table, "  %-34s%s\n", "What you want", "Use this")
	for _, r := range rows {
		cmd := r.cmd
		if strings.HasPrefix(cmd, invoked+" ") {
			cmd = h(cmd)
		}
		fmt.Fprintf(&table, "  %-34s%s\n", r.goal, cmd)
	}

	return fmt.Sprintf("%s v%s\n"+
		"Keep or drop a section of a video by driving ffmpeg.\n"+
		"\n"+
		"%s\n"+
		"  vkeep keeps the part you want. vdrop removes the part you don't want\n"+
		"  (and joins the remainder). They are counterparts — every result is\n"+
		"  reachable from either, but one form is usually shorter.\n"+
		"\n"+
		"%s\n"+
		"  vkeep START [END] [-a] <input>     keep START..END\n"+
		"  vdrop START [END] [-a] <input>     drop START..END, join the rest\n"+
		"\n"+
		"%s\n"+
		"  MM:SS by default (8:31); a bare integer is whole seconds (90); HH:MM:SS\n"+
		"  only when the source is longer than one hour. END is optional — omit it\n"+
		"  (or pass the literal 'end') to reach the source end.\n"+
		"\n"+
		"%s\n"+
		"%s"+
		"\n"+
		"%s\n"+
		"  -a, --accurate         Frame-accurate re-encode (default: fast keyframe copy)\n"+
		"  -x, --crossfade[=SECS] Dissolve the interior join (vdrop only; re-encodes;\n"+
		"                         default 0.5s)\n"+
		"  -v, --version          Show this help message and exit\n"+
		"  -h, -?, --help         Show this help message and exit\n"+
		"\n"+
		"%s\n"+
		"  vkeep 0 FILE copies the whole file. vdrop has no whole-file form —\n"+
		"  dropping 0..end would remove everything, which vdrop refuses.\n"+
		"  A vdrop crossfade (-x) overlaps SECS seconds, so the output is that much\n"+
		"  shorter than a hard cut.\n"+
		"  Requires ffmpeg and ffprobe on PATH (brew install ffmpeg).\n",
		h(invoked), version,
		h("Overview"),
		h("Usage"),
		h("Timestamps"),
		h("Cheatsheet"),
		table.String(),
		h("Options"),
		h("Notes"))
}

// process is the shared driver for Keep and Drop: it validates tooling and input,
// probes duration, resolves and validates the range, refuses to overwrite, then
// executes the byte-copy fast path or the ffmpeg plan and prints the summary.
// crossfade (Drop only, > 0) selects the dissolving interior-join plan.
func process(isCut, accurate bool, crossfade float64, startTok, endTok, input string) error {
	if err := toolsAvailable(); err != nil {
		return err
	}
	if fi, err := os.Stat(input); err != nil || fi.IsDir() {
		return fmt.Errorf("input %q: not a readable file", input)
	}

	duration, err := probeDurationSeconds(input)
	if err != nil {
		return err
	}

	start, err := parseAndGate(startTok, duration)
	if err != nil {
		return err
	}
	end, err := resolveEnd(endTok, duration)
	if err != nil {
		return err
	}

	if isCut {
		err = validateCut(start, end, duration)
	} else {
		err = validateClip(start, end, duration)
	}
	if err != nil {
		return err
	}
	if crossfade > 0 {
		if err := validateCrossfade(start, end, duration, crossfade); err != nil {
			return err
		}
	}

	output := filepath.Join(filepath.Dir(input), deriveOutputName(filepath.Base(input)))
	if _, err := os.Stat(output); err == nil {
		return fmt.Errorf("output %q already exists; refusing to overwrite", output)
	}

	var plan Plan
	switch {
	case !isCut:
		plan = keep(start, end, accurate, input, output, duration)
	case crossfade > 0:
		plan = Plan{Cmds: [][]string{{"-i", input, "-filter_complex", crossfadeFilter(start, end, crossfade), "-map", "[v]", "-map", "[a]", output}}}
	default:
		tmpDir := ""
		interior := start != 0 && end < int(duration)
		if interior && !accurate {
			if tmpDir, err = makeTempDir(); err != nil {
				return fmt.Errorf("creating temp dir: %w", err)
			}
			defer os.RemoveAll(tmpDir)
		}
		plan = cut(start, end, accurate, input, output, tmpDir, duration)
	}

	// An empty plan means a whole-file keep; byte-copy it (-a is moot).
	if len(plan.Cmds) == 0 {
		if err := copyFile(input, output); err != nil {
			return fmt.Errorf("copying %q to %q: %w", input, output, err)
		}
		return printSummary(input, output)
	}

	if plan.ConcatList != "" {
		if err := os.WriteFile(plan.ConcatList, []byte(plan.ConcatBody), 0o644); err != nil {
			return fmt.Errorf("writing concat list: %w", err)
		}
	}
	for _, c := range plan.Cmds {
		if err := runFFmpeg(c); err != nil {
			return fmt.Errorf("ffmpeg failed: %w", err)
		}
	}
	return printSummary(input, output)
}

// copyFile copies src to dst byte-for-byte.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// fileStat holds the summary-table fields for one file.
type fileStat struct {
	label   string
	name    string
	size    int64
	seconds int
}

// gatherStat collects the size and duration of path for the summary table.
func gatherStat(label, path string) (fileStat, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return fileStat{}, err
	}
	d, err := probeDurationSeconds(path)
	if err != nil {
		return fileStat{}, err
	}
	return fileStat{label: label, name: filepath.Base(path), size: fi.Size(), seconds: int(d + 0.5)}, nil
}

// printSummary prints the input-vs-output summary table.
func printSummary(input, output string) error {
	in, err := gatherStat("input", input)
	if err != nil {
		return err
	}
	out, err := gatherStat("output", output)
	if err != nil {
		return err
	}
	fmt.Print(renderSummary([]fileStat{in, out}))
	return nil
}

// renderSummary lays out the FILE/NAME/SIZE/DURATION table for the given rows.
func renderSummary(rows []fileStat) string {
	heads := []string{"FILE", "NAME", "SIZE", "DURATION"}
	right := []bool{false, false, true, true}
	cells := make([][]string, len(rows))
	for i, r := range rows {
		cells[i] = []string{r.label, r.name, formatBytes(r.size), formatDuration(r.seconds)}
	}

	widths := make([]int, len(heads))
	for c, h := range heads {
		widths[c] = len(h)
		for _, row := range cells {
			if len(row[c]) > widths[c] {
				widths[c] = len(row[c])
			}
		}
	}

	var b strings.Builder
	for c, h := range heads {
		if c > 0 {
			b.WriteString("  ")
		}
		b.WriteString(color.Whi10(pad(h, widths[c], right[c])))
	}
	b.WriteByte('\n')
	for _, row := range cells {
		for c, v := range row {
			if c > 0 {
				b.WriteString("  ")
			}
			b.WriteString(pad(v, widths[c], right[c]))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// pad left- or right-justifies s to width w.
func pad(s string, w int, right bool) string {
	if len(s) >= w {
		return s
	}
	sp := strings.Repeat(" ", w-len(s))
	if right {
		return sp + s
	}
	return s + sp
}
