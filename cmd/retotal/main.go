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
	"unicode/utf8"

	"github.com/queone/gkit/internal/color"
	"github.com/queone/gkit/internal/help"
	"github.com/queone/gkit/internal/numfmt"
)

const (
	programName    = "retotal"
	programVersion = "1.3.0"
	// signatureLine gates re-tally and tells the user how to recalculate. It is the
	// last non-empty line of every output file. The `<THIS-FILE>` token is a
	// literal placeholder, so the signature is path-independent.
	signatureLine = "NOTE: To recalculate above TOTAL line, run `retotal <THIS-FILE>`"
	// legacySignatureLine is the earlier signature; re-tally accepts it and
	// rewrites the file with signatureLine.
	legacySignatureLine = "NOTE: To recalculate TOTALS for this FILE, run `retotal <FILE>`"
	// methodPlaceholder fills every data row's empty METHOD entry.
	methodPlaceholder = "-"
)

var outHeader = [5]string{"DESCRIPTION", "MO/AVG", "YR/AVG", "METHOD", "NOTES"}

type row struct {
	typ    string
	desc   string
	mo     string
	yr     string
	method string
	note   string
}

// totals holds the MO/AVG and YR/AVG values of a TOTAL row.
type totals struct {
	mo string
	yr string
}

// span is one entry of an aligned line: its text, its start and end character
// columns (end exclusive), and its start byte offset.
type span struct {
	text      string
	start     int
	end       int
	byteStart int
}

// usageText returns the days-style information screen (program name, version,
// overview, supported invocations).
func helpDoc() help.Doc {
	return help.Doc{
		Name:        programName,
		Version:     programVersion,
		Description: "Consolidate financial data into a signed TOTALS summary, and re-tally it after edits",
		URL:         help.URL(programName),
		Sections: []help.Section{
			{Title: "Usage", Rows: []help.Row{{Form: programName + " FILE", Meaning: "Consolidate FILE into <stem>.txt, or re-tally a signed output file in place"}},
				Lines: []string{
					"FILE is CSV or space-aligned financial data; the consolidation writes an aligned",
					"summary with computed TOTALS and a signature line. When FILE is already a signed",
					"retotal output file, its TOTALS are recomputed in place after you edit it.",
				}},
		},
	}
}

func usageText() string { return helpDoc().String() }

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
		return numfmt.Commas(formatted)
	}
	return numfmt.Commas(strconv.FormatInt(int64(n), 10))
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

// reCell matches one aligned entry: words separated by single spaces.
var reCell = regexp.MustCompile(`[^ ]+(?: [^ ]+)*`)

// cellSpans splits an aligned line into entries separated by two or more
// spaces, recording where each entry starts and ends in character columns.
func cellSpans(line string) []span {
	var out []span
	for _, loc := range reCell.FindAllStringIndex(line, -1) {
		text := line[loc[0]:loc[1]]
		start := utf8.RuneCountInString(line[:loc[0]])
		out = append(out, span{text: text, start: start, end: start + utf8.RuneCountInString(text), byteStart: loc[0]})
	}
	return out
}

// isAmountColumn reports whether a column holds right-aligned amounts.
func isAmountColumn(name string) bool {
	return name == "MO/AVG" || name == "YR/AVG"
}

// alignedRow maps a re-tally line's entries onto header columns by position: a
// text entry must start where its header starts, and an amount must end where
// its header ends. Everything from the NOTES entry to the end of the line is
// the note. ok is false when any entry lines up with no column.
func alignedRow(header []span, line string) (values map[string]string, ok bool) {
	values = map[string]string{}
	for _, c := range cellSpans(line) {
		col := -1
		for j, h := range header {
			if (isAmountColumn(h.text) && c.end == h.end) || (!isAmountColumn(h.text) && c.start == h.start) {
				col = j
				break
			}
		}
		if col < 0 {
			return nil, false
		}
		name := header[col].text
		if name == "NOTES" {
			values[name] = strings.TrimRight(line[c.byteStart:], " \t")
			return values, true
		}
		values[name] = c.text
	}
	return values, true
}

// isAmount reports whether s reads as an amount, ignoring thousands commas.
func isAmount(s string) bool {
	_, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)
	return err == nil && strings.ContainsAny(s, "0123456789")
}

// realignRow places the entries of a row that doesn't line up with the header
// by their shape. The entries before the first amount form the description.
// Up to two consecutive amounts follow; a lone amount goes to the amount
// column whose end is nearer. The first remaining entry is METHOD unless, after
// correcting for how far the amounts drifted, it starts nearer the NOTES
// header; then it starts the note. Anything after METHOD is the note.
func realignRow(header []span, line string) map[string]string {
	cols := map[string]span{}
	for _, h := range header {
		cols[h.text] = h
	}
	cells := cellSpans(line)
	values := map[string]string{}
	if len(cells) == 0 {
		return values
	}
	// raw returns the line from cells[i] through cells[j], spacing intact.
	raw := func(i, j int) string {
		return line[cells[i].byteStart : cells[j].byteStart+len(cells[j].text)]
	}

	descEnd := 1
	if k := slices.IndexFunc(cells[1:], func(c span) bool { return isAmount(c.text) }); k >= 0 {
		descEnd = k + 1
	}
	values["DESCRIPTION"] = raw(0, descEnd-1)

	rest := cells[descEnd:]
	amounts := 0
	for amounts < 2 && amounts < len(rest) && isAmount(rest[amounts].text) {
		amounts++
	}
	shift := 0
	switch amounts {
	case 2:
		values["MO/AVG"], values["YR/AVG"] = rest[0].text, rest[1].text
		shift = rest[1].end - cols["YR/AVG"].end
	case 1:
		name := "YR/AVG"
		if abs(rest[0].end-cols["MO/AVG"].end) < abs(rest[0].end-cols["YR/AVG"].end) {
			name = "MO/AVG"
		}
		values[name] = rest[0].text
		shift = rest[0].end - cols[name].end
	}
	rest = rest[amounts:]
	if len(rest) == 0 {
		return values
	}

	method, hasMethod := cols["METHOD"]
	notes, hasNotes := cols["NOTES"]
	start := rest[0].start - shift
	if !hasMethod || (hasNotes && abs(start-notes.start) < abs(start-method.start)) {
		values["NOTES"] = strings.TrimRight(line[rest[0].byteStart:], " \t")
		return values
	}
	values["METHOD"] = rest[0].text
	if len(rest) > 1 {
		values["NOTES"] = strings.TrimRight(line[rest[1].byteStart:], " \t")
	}
	return values
}

// abs returns the absolute value of n.
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

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
			typ:    get("TYPE"),
			desc:   get("DESCRIPTION"),
			mo:     get("MO/AVG"),
			yr:     get("YR/AVG"),
			method: get("METHOD"),
			note:   get("NOTES"),
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

	// A lone entry after YR/AVG is METHOD only when it starts left of the NOTES
	// header; otherwise it is the note.
	headerSpans := cellSpans(lines[0])
	mi, hasMethod := colIdx["METHOD"]
	ni, hasNotes := colIdx["NOTES"]
	loneEntryRule := hasMethod && hasNotes && mi == ni-1 && ni == ncols-1 && len(headerSpans) == ncols

	var rows []row
	for _, line := range lines[1:] {
		values := splitAligned(line, ncols)
		if loneEntryRule {
			if cells := cellSpans(line); len(cells) == ni && cells[ni-1].start >= headerSpans[ni].start {
				values[ni], values[mi] = values[mi], ""
			}
		}
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
			typ:    typ,
			desc:   desc,
			mo:     get("MO/AVG"),
			yr:     get("YR/AVG"),
			method: get("METHOD"),
			note:   get("NOTES"),
		})
	}
	return rows, nil
}

// readRetotalOutput parses retotal output-format content into rows, skipping
// signature lines and total-bearing rows. It returns the last TOTAL row's
// values, or nil when there is none, and one "line N: <first entry>" item for
// each row that didn't line up with the header and was placed by realignRow.
func readRetotalOutput(content string) (rows []row, prior *totals, realigned []string) {
	var header []span
	for i, ln := range strings.Split(content, "\n") {
		ln = strings.TrimRight(ln, "\r")
		if strings.TrimSpace(ln) == "" || isSignature(ln) {
			continue
		}
		if header == nil {
			header = cellSpans(ln)
			continue
		}

		values, ok := alignedRow(header, ln)
		if !ok {
			values = realignRow(header, ln)
			realigned = append(realigned, fmt.Sprintf("line %d: %s", i+1, cellSpans(ln)[0].text))
		}

		desc := values["DESCRIPTION"]
		if strings.EqualFold(desc, "total") {
			prior = &totals{mo: values["MO/AVG"], yr: values["YR/AVG"]}
			continue
		}
		if strings.Contains(strings.ToLower(desc), "total") {
			continue
		}

		rows = append(rows, row{
			desc:   desc,
			mo:     values["MO/AVG"],
			yr:     values["YR/AVG"],
			method: values["METHOD"],
			note:   values["NOTES"],
		})
	}
	return rows, prior, realigned
}

// realignNotice reports realigned rows so the user can check how they were
// placed.
func realignNotice(realigned []string) string {
	noun, pronoun := "rows", "them"
	if len(realigned) == 1 {
		noun, pronoun = "row", "it"
	}
	return fmt.Sprintf("%s: realigned %d %s whose spacing had drifted; check %s:\n  %s",
		programName, len(realigned), noun, pronoun, strings.Join(realigned, "\n  "))
}

// hasSignature reports whether content's last non-empty line is the signature
// line or the legacy signature line (trailing whitespace tolerated).
func hasSignature(content string) bool {
	lines := strings.Split(content, "\n")
	for _, line := range slices.Backward(lines) {
		if strings.TrimSpace(line) == "" {
			continue
		}
		return isSignature(line)
	}
	return false
}

// isSignature reports whether line, ignoring trailing whitespace, is the
// signature line or the legacy signature line.
func isSignature(line string) bool {
	t := strings.TrimRight(line, " \t\r")
	return t == signatureLine || t == legacySignatureLine
}

// stemTxt derives the consolidation output filename by replacing path's
// extension with .txt.
func stemTxt(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + ".txt"
}

type outputRow [5]string

// methodOrPlaceholder returns method, or methodPlaceholder when it is empty.
func methodOrPlaceholder(method string) string {
	if strings.TrimSpace(method) == "" {
		return methodPlaceholder
	}
	return method
}

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
		out = append(out, outputRow{desc, commatize(normalize2(mo)), commatize(normalize2(yr)), methodOrPlaceholder(r.method), r.note})
	}

	out = append(out, outputRow{
		"TOTAL",
		commatize(fmt.Sprintf("%.2f", moTotal)),
		commatize(fmt.Sprintf("%.2f", yrTotal)),
		"",
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

		out = append(out, outputRow{r.desc, commatize(normalize2(mo)), commatize(normalize2(yr)), methodOrPlaceholder(r.method), r.note})
	}

	out = append(out, outputRow{
		"TOTAL",
		commatize(fmt.Sprintf("%.2f", moTotal)),
		commatize(fmt.Sprintf("%.2f", yrTotal)),
		"",
		"",
	})
	return out
}

// cents returns an amount rounded to whole cents, for comparing totals.
func cents(s string) float64 {
	return math.Round(toFloat(s) * 100)
}

// totalChanges returns one "<column> TOTAL: <old> -> <new>" line per TOTAL
// value that differs from prior at cent precision. A missing prior TOTAL row
// reads "none".
func totalChanges(prior *totals, total outputRow) []string {
	var old [2]string
	if prior != nil {
		old = [2]string{prior.mo, prior.yr}
	}
	var lines []string
	for i, name := range [2]string{"MO/AVG", "YR/AVG"} {
		now := total[i+1]
		was := "none"
		if prior != nil {
			if cents(old[i]) == cents(now) {
				continue
			}
			was = commatize(fmt.Sprintf("%.2f", toFloat(old[i])))
		}
		lines = append(lines, fmt.Sprintf("%s TOTAL: %s -> %s", name, was, now))
	}
	return lines
}

// formatOutput renders the header and rows as an aligned table. Widths count
// characters, not bytes, so non-ASCII text lines up.
func formatOutput(rows []outputRow) string {
	widths := [5]int{}
	for i, h := range outHeader {
		widths[i] = max(widths[i], utf8.RuneCountInString(h))
	}
	for _, r := range rows {
		for i, v := range r {
			widths[i] = max(widths[i], utf8.RuneCountInString(v))
		}
	}

	rightAlign := [5]bool{false, true, true, false, false}

	pad := func(s string, w int, right bool) string {
		if right {
			return fmt.Sprintf("%*s", w, s)
		}
		return fmt.Sprintf("%-*s", w, s)
	}

	emit := func(vals [5]string) string {
		parts := make([]string, 5)
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
// and rewrites the file in place, lined up. It reports realigned rows in
// yellow on stderr and prints each TOTAL value that changed.
func retally(inPath string) error {
	data, err := os.ReadFile(inPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", inPath, err)
	}
	content := stripBOM(string(data))
	if !hasSignature(content) {
		return fmt.Errorf("%s is missing the required signature line; add this as the last line of the file:\n%s", inPath, signatureLine)
	}

	input, prior, realigned := readRetotalOutput(content)
	out := processRetally(input)
	result := withSignature(formatOutput(out))
	if err := os.WriteFile(inPath, []byte(result), 0644); err != nil {
		return fmt.Errorf("write %s: %w", inPath, err)
	}
	if len(realigned) > 0 {
		fmt.Fprintln(os.Stderr, color.Yel5(realignNotice(realigned)))
	}
	for _, line := range totalChanges(prior, out[len(out)-1]) {
		fmt.Println(line)
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

	if len(args) == 1 && (args[0] == "-v" || args[0] == "--version") {
		fmt.Printf("%s v%s\n", programName, programVersion)
		return nil
	}
	if len(args) != 1 || args[0] == "-h" || args[0] == "-?" || args[0] == "--help" {
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
