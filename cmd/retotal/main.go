package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/queone/governa-color"
)

const (
	programName    = "retotal"
	programVersion = "1.0.0"
	// signatureLine gates re-tally and tells the user how to recalculate. It is the
	// last non-empty line of every output file. The `<FILE>` token is a literal
	// placeholder, so the signature is path-independent.
	signatureLine = "NOTE: To recalculate TOTALS for this FILE, run `retotal <FILE>`"
)

var outHeader = [4]string{"DESCRIPTION", "MO/AVG", "YR/AVG", "NOTES"}

type row struct {
	typ  string
	desc string
	mo   string
	yr   string
	note string
}

// usageText returns the days-style information screen (program name, version,
// overview, supported invocations).
func usageText() string {
	n := color.Whi10(programName)
	return fmt.Sprintf("%s v%s\n"+
		"Financial TOTALS consolidator and re-tallier — https://github.com/queone/gkit/blob/main/cmd/retotal/README.md\n"+
		"%s\n"+
		"  retotal reads CSV or space-aligned financial data and writes an aligned summary with computed\n"+
		"  TOTALS, signed with a recalculation note; it then re-tallies that signed output file in place\n"+
		"  after you edit it. Supported invocations are:\n"+
		"\n"+
		"    retotal -h, --help    Prints this information screen.\n"+
		"    retotal FILE          Consolidate CSV/aligned input into <stem>.txt with computed TOTALS and a\n"+
		"                          signature line; or, when FILE is already a signed retotal output file,\n"+
		"                          recompute its TOTALS in place.\n",
		n, programVersion, color.Whi10("Overview"))
}

// printUsage prints the information screen and exits 0.
func printUsage() {
	fmt.Print(usageText())
	os.Exit(0)
}

func stripBOM(s string) string {
	return strings.TrimPrefix(s, "\xef\xbb\xbf")
}

func commatize(s string) string {
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return s
	}
	if math.Abs(n) < 1000 {
		return s
	}
	if strings.Contains(s, ".") {
		parts := strings.SplitN(s, ".", 2)
		decimals := len(parts[1])
		formatted := strconv.FormatFloat(n, 'f', decimals, 64)
		return addCommas(formatted)
	}
	return addCommas(strconv.FormatInt(int64(n), 10))
}

func addCommas(s string) string {
	negative := false
	if strings.HasPrefix(s, "-") {
		negative = true
		s = s[1:]
	}

	intPart := s
	decPart := ""
	if idx := strings.Index(s, "."); idx >= 0 {
		intPart = s[:idx]
		decPart = s[idx:]
	}

	var b strings.Builder
	for i, ch := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(ch)
	}

	result := b.String() + decPart
	if negative {
		result = "-" + result
	}
	return result
}

func normalize2(s string) string {
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return s
	}
	return fmt.Sprintf("%.2f", n)
}

func toFloat(s string) float64 {
	s = strings.ReplaceAll(s, ",", "")
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return n
}

var reQuoted = regexp.MustCompile(`"[^"]*"`)
var reTwoSpaces = regexp.MustCompile(` {2,}`)

// isRetotalOutput reports whether path's first non-empty line is the retotal
// output header (DESCRIPTION / MO/AVG / YR/AVG), selecting the re-tally path.
func isRetotalOutput(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	content := stripBOM(string(data))
	for _, ln := range strings.SplitN(content, "\n", 2) {
		line := strings.TrimSpace(ln)
		if line == "" {
			continue
		}
		cols := reTwoSpaces.Split(line, -1)
		if len(cols) >= 3 && cols[0] == "DESCRIPTION" && cols[1] == "MO/AVG" && cols[2] == "YR/AVG" {
			return true, nil
		}
		return false, nil
	}
	return false, nil
}

func detectInputFormat(firstLine string) string {
	stripped := reQuoted.ReplaceAllString(firstLine, "")
	if strings.Contains(stripped, ",") {
		return "csv"
	}
	if reTwoSpaces.MatchString(firstLine) {
		return "aligned"
	}
	return "csv"
}

func readCSV(path string) ([]row, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	content := stripBOM(string(data))

	r := csv.NewReader(strings.NewReader(content))
	headers, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV headers from %s: %w", path, err)
	}
	for i := range headers {
		headers[i] = strings.ToUpper(strings.TrimSpace(headers[i]))
	}

	colIdx := map[string]int{}
	for i, h := range headers {
		colIdx[h] = i
	}

	var rows []row
	for {
		record, err := r.Read()
		if err != nil {
			break
		}
		get := func(name string) string {
			if idx, ok := colIdx[name]; ok && idx < len(record) {
				return strings.TrimSpace(record[idx])
			}
			return ""
		}
		rows = append(rows, row{
			typ:  get("TYPE"),
			desc: get("DESCRIPTION"),
			mo:   get("MO/AVG"),
			yr:   get("YR/AVG"),
			note: get("NOTES"),
		})
	}
	return rows, nil
}

func splitAligned(line string, ncols int) []string {
	parts := reTwoSpaces.Split(strings.TrimSpace(line), -1)
	for len(parts) < ncols {
		parts = append(parts, "")
	}
	if len(parts) > ncols {
		tail := strings.Join(parts[ncols-1:], "  ")
		parts = append(parts[:ncols-1], tail)
	}
	return parts
}

func readAligned5(path string) ([]row, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	content := stripBOM(string(data))

	var lines []string
	for ln := range strings.SplitSeq(content, "\n") {
		if strings.TrimSpace(ln) != "" {
			lines = append(lines, ln)
		}
	}
	if len(lines) == 0 {
		return nil, nil
	}

	headers := reTwoSpaces.Split(strings.TrimSpace(lines[0]), -1)
	ncols := len(headers)
	colIdx := map[string]int{}
	for i, h := range headers {
		colIdx[h] = i
	}

	var rows []row
	for _, line := range lines[1:] {
		values := splitAligned(line, ncols)
		get := func(name string) string {
			if idx, ok := colIdx[name]; ok && idx < len(values) {
				return values[idx]
			}
			return ""
		}

		desc := get("DESCRIPTION")
		var typ string
		if idx := strings.Index(desc, " - "); idx >= 0 {
			typ = desc[:idx]
			desc = desc[idx+3:]
		}

		rows = append(rows, row{
			typ:  typ,
			desc: desc,
			mo:   get("MO/AVG"),
			yr:   get("YR/AVG"),
			note: get("NOTES"),
		})
	}
	return rows, nil
}

// readRetotalOutput parses a retotal output-format file into rows, skipping the
// trailing signature line and any prior TOTAL row.
func readRetotalOutput(path string) ([]row, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	content := stripBOM(string(data))

	var lines []string
	for ln := range strings.SplitSeq(content, "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		if strings.TrimRight(ln, " \t\r") == signatureLine {
			continue
		}
		lines = append(lines, ln)
	}
	if len(lines) < 2 {
		return nil, nil
	}

	headers := reTwoSpaces.Split(strings.TrimSpace(lines[0]), -1)
	ncols := len(headers)
	colIdx := map[string]int{}
	for i, h := range headers {
		colIdx[h] = i
	}

	var rows []row
	for _, line := range lines[1:] {
		values := splitAligned(line, ncols)
		get := func(name string) string {
			if idx, ok := colIdx[name]; ok && idx < len(values) {
				return values[idx]
			}
			return ""
		}

		desc := get("DESCRIPTION")
		if strings.ToLower(desc) == "total" || strings.Contains(strings.ToLower(desc), "total") {
			continue
		}

		rows = append(rows, row{
			desc: desc,
			mo:   get("MO/AVG"),
			yr:   get("YR/AVG"),
			note: get("NOTES"),
		})
	}
	return rows, nil
}

// hasSignature reports whether content's last non-empty line is the signature
// line (trailing whitespace tolerated).
func hasSignature(content string) bool {
	lines := strings.Split(content, "\n")
	for _, line := range slices.Backward(lines) {
		t := strings.TrimRight(line, " \t\r")
		if strings.TrimSpace(t) == "" {
			continue
		}
		return t == signatureLine
	}
	return false
}

// stemTxt derives the consolidation output filename by replacing path's
// extension with .txt.
func stemTxt(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + ".txt"
}

type outputRow [4]string

func process(input []row) []outputRow {
	var out []outputRow
	var moTotal, yrTotal float64

	for _, r := range input {
		if strings.Contains(strings.ToLower(r.typ), "total") ||
			strings.Contains(strings.ToLower(r.desc), "total") {
			continue
		}
		if strings.ToLower(r.typ) == "type" && strings.ToLower(r.desc) == "description" {
			continue
		}

		mo := strings.ReplaceAll(r.mo, ",", "")
		yr := strings.ReplaceAll(r.yr, ",", "")
		moTotal += toFloat(mo)
		yrTotal += toFloat(yr)

		desc := r.desc
		if r.typ != "" {
			desc = r.typ + " - " + r.desc
		}
		out = append(out, outputRow{desc, commatize(normalize2(mo)), commatize(normalize2(yr)), r.note})
	}

	out = append(out, outputRow{
		"TOTAL",
		commatize(fmt.Sprintf("%.2f", moTotal)),
		commatize(fmt.Sprintf("%.2f", yrTotal)),
		"",
	})
	return out
}

func processRetally(input []row) []outputRow {
	var out []outputRow
	var moTotal, yrTotal float64

	for _, r := range input {
		mo := strings.ReplaceAll(r.mo, ",", "")
		yr := strings.ReplaceAll(r.yr, ",", "")
		moTotal += toFloat(mo)
		yrTotal += toFloat(yr)

		out = append(out, outputRow{r.desc, commatize(normalize2(mo)), commatize(normalize2(yr)), r.note})
	}

	out = append(out, outputRow{
		"TOTAL",
		commatize(fmt.Sprintf("%.2f", moTotal)),
		commatize(fmt.Sprintf("%.2f", yrTotal)),
		"",
	})
	return out
}

func formatOutput(rows []outputRow) string {
	widths := [4]int{}
	for i, h := range outHeader {
		if len(h) > widths[i] {
			widths[i] = len(h)
		}
	}
	for _, r := range rows {
		for i, v := range r {
			if len(v) > widths[i] {
				widths[i] = len(v)
			}
		}
	}

	rightAlign := [4]bool{false, true, true, false}

	pad := func(s string, w int, right bool) string {
		if right {
			return fmt.Sprintf("%*s", w, s)
		}
		return fmt.Sprintf("%-*s", w, s)
	}

	emit := func(vals [4]string) string {
		parts := make([]string, 4)
		for i, v := range vals {
			parts[i] = pad(v, widths[i], rightAlign[i])
		}
		return strings.TrimRight(strings.Join(parts, "  "), " ")
	}

	var b strings.Builder
	b.WriteString(emit(outHeader))
	b.WriteByte('\n')
	for _, r := range rows {
		b.WriteString(emit(r))
		b.WriteByte('\n')
	}
	return b.String()
}

// withSignature appends a blank separator line and the signature to a formatted
// table.
func withSignature(table string) string {
	return table + "\n" + signatureLine + "\n"
}

// retally validates the signature on an output-format file, recomputes TOTAL,
// and rewrites the file in place.
func retally(inPath string) error {
	data, err := os.ReadFile(inPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", inPath, err)
	}
	if !hasSignature(stripBOM(string(data))) {
		return fmt.Errorf("%s is missing the required signature line; add this as the last line of the file:\n%s", inPath, signatureLine)
	}

	input, err := readRetotalOutput(inPath)
	if err != nil {
		return err
	}
	result := withSignature(formatOutput(processRetally(input)))
	if err := os.WriteFile(inPath, []byte(result), 0644); err != nil {
		return fmt.Errorf("write %s: %w", inPath, err)
	}
	return nil
}

// consolidate reads CSV/aligned input, writes a signed <stem>.txt summary, and
// prints a recalculation hint.
func consolidate(inPath string) error {
	outPath := stemTxt(inPath)
	if outPath == inPath {
		return fmt.Errorf("input %s already uses the .txt output name; rename the input first", inPath)
	}
	if _, err := os.Stat(outPath); err == nil {
		return fmt.Errorf("%s already exists; remove or rename it first", outPath)
	}

	data, err := os.ReadFile(inPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", inPath, err)
	}
	firstLine := ""
	for _, ln := range strings.SplitN(stripBOM(string(data)), "\n", 2) {
		if strings.TrimSpace(ln) != "" {
			firstLine = stripBOM(ln)
			break
		}
	}

	var input []row
	if detectInputFormat(firstLine) == "csv" {
		input, err = readCSV(inPath)
	} else {
		input, err = readAligned5(inPath)
	}
	if err != nil {
		return err
	}

	result := withSignature(formatOutput(process(input)))
	if err := os.WriteFile(outPath, []byte(result), 0644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	fmt.Printf("wrote %s; run `retotal %s` to recalculate TOTALS\n", outPath, outPath)
	return nil
}

func run() error {
	args := os.Args[1:]

	if len(args) != 1 || args[0] == "-h" || args[0] == "--help" {
		printUsage()
	}

	inPath := args[0]

	isOutput, err := isRetotalOutput(inPath)
	if err != nil {
		return err
	}
	if isOutput {
		return retally(inPath)
	}
	return consolidate(inPath)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
		os.Exit(1)
	}
}
