package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCommatize(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"100", "100"},
		{"999", "999"},
		{"999.99", "999.99"},
		{"1000", "1,000"},
		{"1234", "1,234"},
		{"1234567", "1,234,567"},
		{"1234.56", "1,234.56"},
		{"12345.678", "12,345.678"},
		{"-1500", "-1,500"},
		{"-1500.25", "-1,500.25"},
		{"0", "0"},
		{"abc", "abc"},
		{"", ""},
	}
	for _, tt := range tests {
		got := commatize(tt.in)
		if got != tt.want {
			t.Errorf("commatize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"1234", 1234.0},
		{"1,234", 1234.0},
		{"1,234.56", 1234.56},
		{"abc", 0.0},
		{"", 0.0},
	}
	for _, tt := range tests {
		got := toFloat(tt.in)
		if got != tt.want {
			t.Errorf("toFloat(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestIsRetotalOutput(t *testing.T) {
	dir := t.TempDir()

	mco := filepath.Join(dir, "output.txt")
	os.WriteFile(mco, []byte("DESCRIPTION  MO/AVG  YR/AVG  NOTES\nRent  1,200.00  14,400.00\n"), 0644)
	got, err := isRetotalOutput(mco)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected retotal output format to be detected")
	}

	csvf := filepath.Join(dir, "input.csv")
	os.WriteFile(csvf, []byte("TYPE,DESCRIPTION,MO/AVG,YR/AVG,NOTES\nIncome,Salary,5000,60000,\n"), 0644)
	got, err = isRetotalOutput(csvf)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("expected CSV to not be detected as retotal output")
	}

	aligned := filepath.Join(dir, "aligned.dat")
	os.WriteFile(aligned, []byte("DESCRIPTION  MO/AVG  YR/AVG  NOTES\nIncome - Salary  5000  60000\n"), 0644)
	got, err = isRetotalOutput(aligned)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected 4-column aligned with DESCRIPTION header to be detected as retotal output")
	}
}

// captureRun runs the program in dir with args, capturing stdout, and returns
// stdout plus run's error.
func captureRun(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	os.Args = append([]string{"retotal"}, args...)

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := run()
	w.Close()
	os.Stdout = oldStdout
	out, _ := io.ReadAll(r)
	return string(out), err
}

func runInDir(t *testing.T, dir string, args ...string) error {
	t.Helper()
	_, err := captureRun(t, dir, args...)
	return err
}

// signed wraps table content with a blank separator and the signature line.
func signed(table string) string {
	return table + "\n" + signatureLine + "\n"
}

// totalRow returns the TOTAL row from a result string.
func totalRow(t *testing.T, result string) string {
	t.Helper()
	for ln := range strings.SplitSeq(result, "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "TOTAL") {
			return ln
		}
	}
	t.Fatalf("no TOTAL row in:\n%s", result)
	return ""
}

func TestCSVInput(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.csv")

	csv := "TYPE,DESCRIPTION,MO/AVG,YR/AVG,NOTES\n" +
		"Income,Salary,5000,60000,primary\n" +
		"Income,Freelance,1500,18000,\n"
	os.WriteFile(in, []byte(csv), 0644)

	if err := runInDir(t, dir, in); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "in.txt"))
	result := string(data)

	if !strings.Contains(result, "Income - Salary") {
		t.Error("expected TYPE merged into DESCRIPTION as 'Income - Salary'")
	}
	if !strings.Contains(result, "Income - Freelance") {
		t.Error("expected TYPE merged into DESCRIPTION as 'Income - Freelance'")
	}
	if !strings.Contains(result, "TOTAL") {
		t.Error("expected TOTAL row")
	}
	if !strings.Contains(result, "6,500.00") {
		t.Errorf("expected MO total 6,500.00 in output:\n%s", result)
	}
	if !strings.Contains(result, "78,000.00") {
		t.Errorf("expected YR total 78,000.00 in output:\n%s", result)
	}
}

func TestAlignedInput(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.dat")

	aligned := "DESCRIPTION  TYPE  MO/AVG  YR/AVG  NOTES\n" +
		"Income - Salary  Inc  5000  60000  primary\n" +
		"Income - Freelance  Inc  1500  18000\n"
	os.WriteFile(in, []byte(aligned), 0644)

	if err := runInDir(t, dir, in); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "in.txt"))
	result := string(data)

	if !strings.Contains(result, "Income - Salary") {
		t.Error("expected TYPE extracted and merged back as 'Income - Salary'")
	}
	if !strings.Contains(result, "6,500.00") {
		t.Errorf("expected MO total 6,500.00 in output:\n%s", result)
	}
}

func TestSkipTotalAndHeaderRows(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.csv")

	csv := "TYPE,DESCRIPTION,MO/AVG,YR/AVG,NOTES\n" +
		"TYPE,DESCRIPTION,MO/AVG,YR/AVG,NOTES\n" +
		"Income,Salary,5000,60000,\n" +
		"Total,All Income,5000,60000,\n" +
		",Grand total,5000,60000,\n"
	os.WriteFile(in, []byte(csv), 0644)

	if err := runInDir(t, dir, in); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "in.txt"))
	result := string(data)

	// One data row (Salary) + TOTAL; the duplicate header and the two total-bearing
	// rows are dropped.
	dataRows := 0
	for ln := range strings.SplitSeq(strings.TrimRight(result, "\n"), "\n") {
		s := strings.TrimSpace(ln)
		if s == "" || s == signatureLine || strings.HasPrefix(s, "DESCRIPTION") {
			continue
		}
		dataRows++
	}
	if dataRows != 2 {
		t.Errorf("expected 2 rows (1 data + TOTAL), got %d:\n%s", dataRows, result)
	}
	if !strings.Contains(result, "5,000.00") {
		t.Errorf("expected MO total 5,000.00 (only Salary counted):\n%s", result)
	}
}

func TestEmptyFields(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.csv")

	csv := "TYPE,DESCRIPTION,MO/AVG,YR/AVG,NOTES\n" +
		",Rent,1200,14400,\n" +
		",Groceries,,3600,\n"
	os.WriteFile(in, []byte(csv), 0644)

	if err := runInDir(t, dir, in); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "in.txt"))
	result := string(data)

	if !strings.Contains(result, "Rent") {
		t.Error("expected Rent row without TYPE prefix")
	}
	if !strings.Contains(result, "1,200.00") {
		t.Errorf("expected MO total 1,200.00:\n%s", result)
	}
	if !strings.Contains(result, "18,000.00") {
		t.Errorf("expected YR total 18,000.00:\n%s", result)
	}
}

func TestPrecommatizedInput(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.csv")

	csv := "TYPE,DESCRIPTION,MO/AVG,YR/AVG,NOTES\n" +
		"Income,Salary,\"5,000\",\"60,000\",\n"
	os.WriteFile(in, []byte(csv), 0644)

	if err := runInDir(t, dir, in); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "in.txt"))
	result := string(data)

	if !strings.Contains(result, "5,000.00") {
		t.Errorf("expected data row MO 5,000.00:\n%s", result)
	}
	if !strings.Contains(result, "60,000.00") {
		t.Errorf("expected data row YR 60,000.00:\n%s", result)
	}
}

func TestOutputAlignment(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.csv")

	csv := "TYPE,DESCRIPTION,MO/AVG,YR/AVG,NOTES\n" +
		"Income,Salary,5000,60000,primary\n" +
		",Rent,1200,14400,monthly\n"
	os.WriteFile(in, []byte(csv), 0644)

	if err := runInDir(t, dir, in); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "in.txt"))
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")

	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}

	for i, line := range lines {
		if line != strings.TrimRight(line, " ") {
			t.Errorf("line %d has trailing spaces: %q", i, line)
		}
	}
}

func TestAllNumericTwoDecimalPlaces(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.csv")

	csv := "TYPE,DESCRIPTION,MO/AVG,YR/AVG,NOTES\n" +
		",Item,100,1200,\n"
	os.WriteFile(in, []byte(csv), 0644)

	if err := runInDir(t, dir, in); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "in.txt"))
	result := string(data)
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")

	dataLine := lines[1]
	if !strings.Contains(dataLine, "100.00") {
		t.Errorf("expected data row MO with 2 decimal places (100.00), got: %s", dataLine)
	}
	if !strings.Contains(dataLine, "1,200.00") {
		t.Errorf("expected data row YR with 2 decimal places (1,200.00), got: %s", dataLine)
	}

	tl := totalRow(t, result)
	if !strings.Contains(tl, "100.00") {
		t.Errorf("expected TOTAL MO with 2 decimal places (100.00), got: %s", tl)
	}
	if !strings.Contains(tl, "1,200.00") {
		t.Errorf("expected TOTAL YR with 2 decimal places (1,200.00), got: %s", tl)
	}
}

func TestCSVMixedCaseHeaders(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.csv")

	csv := "Type,description,mo/avg,yr/avg,Notes\n" +
		"Home,Property taxes,413.26,\"4,959.16\",Quarterly\n" +
		",Groceries,600,7200,\n"
	os.WriteFile(in, []byte(csv), 0644)

	if err := runInDir(t, dir, in); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "in.txt"))
	result := string(data)

	if !strings.Contains(result, "Home - Property taxes") {
		t.Errorf("expected TYPE merged into DESCRIPTION as 'Home - Property taxes':\n%s", result)
	}
	if !strings.Contains(result, "413.26") {
		t.Errorf("expected MO 413.26:\n%s", result)
	}
	if !strings.Contains(result, "4,959.16") {
		t.Errorf("expected YR 4,959.16:\n%s", result)
	}
	if !strings.Contains(result, "Quarterly") {
		t.Errorf("expected NOTES 'Quarterly':\n%s", result)
	}
	if !strings.Contains(result, "1,013.26") {
		t.Errorf("expected MO total 1,013.26:\n%s", result)
	}
}

func TestConsolidationStemOutputAndSignature(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "budget.csv")
	os.WriteFile(in, []byte("TYPE,DESCRIPTION,MO/AVG,YR/AVG,NOTES\nIncome,Salary,5000,60000,primary\n,Rent,1200,14400,monthly\n"), 0644)

	out, err := captureRun(t, dir, in)
	if err != nil {
		t.Fatal(err)
	}

	data, e := os.ReadFile(filepath.Join(dir, "budget.txt"))
	if e != nil {
		t.Fatalf("expected stem-based output budget.txt: %v", e)
	}
	result := string(data)

	if !strings.HasSuffix(strings.TrimRight(result, "\n"), signatureLine) {
		t.Errorf("output should end with the signature line:\n%s", result)
	}
	if !strings.Contains(result, "\n\n"+signatureLine) {
		t.Errorf("expected a blank line before the signature:\n%s", result)
	}
	if !strings.Contains(out, "budget.txt") {
		t.Errorf("hint should name budget.txt, got: %q", out)
	}
}

func TestConsolidationNoClobber(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "budget.csv")
	os.WriteFile(in, []byte("TYPE,DESCRIPTION,MO/AVG,YR/AVG,NOTES\n,Rent,1200,14400,\n"), 0644)
	os.WriteFile(filepath.Join(dir, "budget.txt"), []byte("existing"), 0644)

	err := runInDir(t, dir, in)
	if err == nil {
		t.Fatal("expected error when budget.txt already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestRetallyMode(t *testing.T) {
	dir := t.TempDir()
	budget := filepath.Join(dir, "budget.txt")

	content := signed("DESCRIPTION  MO/AVG  YR/AVG  NOTES\n" +
		"Income - Salary  5000  60000  primary\n" +
		"Rent  1200  14400  monthly\n" +
		"Groceries  600  7200\n" +
		"TOTAL  6800  81600\n")
	os.WriteFile(budget, []byte(content), 0644)

	if err := runInDir(t, dir, budget); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(budget)
	result := string(data)

	if !strings.Contains(result, "5,000.00") {
		t.Errorf("expected normalized Salary MO 5,000.00:\n%s", result)
	}
	if !strings.Contains(result, "60,000.00") {
		t.Errorf("expected normalized Salary YR 60,000.00:\n%s", result)
	}
	if !strings.Contains(result, "1,200.00") {
		t.Errorf("expected normalized Rent MO 1,200.00:\n%s", result)
	}
	if !strings.Contains(result, "600.00") {
		t.Errorf("expected normalized Groceries MO 600.00:\n%s", result)
	}

	tl := totalRow(t, result)
	if !strings.Contains(tl, "6,800.00") {
		t.Errorf("expected recomputed MO total 6,800.00:\n%s", result)
	}
	if !strings.Contains(tl, "81,600.00") {
		t.Errorf("expected recomputed YR total 81,600.00:\n%s", result)
	}
}

func TestRetallyDropsOldTotal(t *testing.T) {
	dir := t.TempDir()
	budget := filepath.Join(dir, "budget.txt")

	content := signed("DESCRIPTION  MO/AVG  YR/AVG  NOTES\n" +
		"Rent  1200  14400  monthly\n" +
		"TOTAL  9999  99999\n")
	os.WriteFile(budget, []byte(content), 0644)

	if err := runInDir(t, dir, budget); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(budget)
	result := string(data)
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")

	totalCount := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "TOTAL") {
			totalCount++
		}
	}
	if totalCount != 1 {
		t.Errorf("expected exactly 1 TOTAL row, got %d:\n%s", totalCount, result)
	}

	tl := totalRow(t, result)
	if !strings.Contains(tl, "1,200.00") {
		t.Errorf("expected recomputed MO total 1,200.00 (not old 9999):\n%s", result)
	}
	if !strings.Contains(tl, "14,400.00") {
		t.Errorf("expected recomputed YR total 14,400.00 (not old 99999):\n%s", result)
	}
}

func TestRetallySloppyEntries(t *testing.T) {
	dir := t.TempDir()
	budget := filepath.Join(dir, "budget.txt")

	content := signed("DESCRIPTION  MO/AVG  YR/AVG  NOTES\n" +
		"Rent  1200  14400  monthly\n" +
		"Groceries  600.5  7206\n" +
		"Internet  89  1068\n")
	os.WriteFile(budget, []byte(content), 0644)

	if err := runInDir(t, dir, budget); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(budget)
	result := string(data)

	if !strings.Contains(result, "1,200.00") {
		t.Errorf("expected Rent MO normalized to 1,200.00:\n%s", result)
	}
	if !strings.Contains(result, "600.50") {
		t.Errorf("expected Groceries MO normalized to 600.50:\n%s", result)
	}
	if !strings.Contains(result, "89.00") {
		t.Errorf("expected Internet MO normalized to 89.00:\n%s", result)
	}
	if !strings.Contains(result, "1,889.50") {
		t.Errorf("expected recomputed MO total 1,889.50:\n%s", result)
	}
	if !strings.Contains(result, "22,674.00") {
		t.Errorf("expected recomputed YR total 22,674.00:\n%s", result)
	}
}

func TestRetallyRequiresSignature(t *testing.T) {
	dir := t.TempDir()

	// Missing signature.
	missing := filepath.Join(dir, "missing.txt")
	body := "DESCRIPTION  MO/AVG  YR/AVG  NOTES\nRent  1200  14400  monthly\nTOTAL  1200  14400\n"
	os.WriteFile(missing, []byte(body), 0644)
	before, _ := os.ReadFile(missing)

	err := runInDir(t, dir, missing)
	if err == nil {
		t.Fatal("expected error for a missing signature")
	}
	if !strings.Contains(err.Error(), signatureLine) {
		t.Errorf("error should contain the verbatim signature line, got: %v", err)
	}
	after, _ := os.ReadFile(missing)
	if string(before) != string(after) {
		t.Error("input file must not be modified when the signature is missing")
	}

	// Altered signature.
	altered := filepath.Join(dir, "altered.txt")
	os.WriteFile(altered, []byte(body+"\nNOTE: recalc with retotal please\n"), 0644)
	err = runInDir(t, dir, altered)
	if err == nil {
		t.Fatal("expected error for an altered signature")
	}
	if !strings.Contains(err.Error(), signatureLine) {
		t.Errorf("error should contain the verbatim signature line, got: %v", err)
	}
}

func TestRetallyIdempotent(t *testing.T) {
	dir := t.TempDir()
	budget := filepath.Join(dir, "b.txt")
	os.WriteFile(budget, []byte(signed("DESCRIPTION  MO/AVG  YR/AVG  NOTES\n"+
		"Rent  1200  14400  monthly\nTOTAL  1200  14400\n")), 0644)

	if err := runInDir(t, dir, budget); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(budget)

	if err := runInDir(t, dir, budget); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(budget)

	if string(first) != string(second) {
		t.Errorf("re-tally not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if strings.Count(string(second), signatureLine) != 1 {
		t.Errorf("expected exactly one signature line:\n%s", second)
	}
	if !strings.HasSuffix(strings.TrimRight(string(second), "\n"), signatureLine) {
		t.Errorf("file should end with the signature:\n%s", second)
	}
}

func TestSignatureNotCountedAsDataRow(t *testing.T) {
	dir := t.TempDir()
	budget := filepath.Join(dir, "b.txt")
	os.WriteFile(budget, []byte(signed("DESCRIPTION  MO/AVG  YR/AVG  NOTES\n"+
		"Rent  1200  14400  monthly\nGroceries  600  7200\nTOTAL  0  0\n")), 0644)

	if err := runInDir(t, dir, budget); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(budget)
	result := string(data)

	// Signature appears exactly once, only as the trailing line.
	if strings.Count(result, "NOTE: To recalculate") != 1 {
		t.Errorf("signature should appear exactly once:\n%s", result)
	}
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	for i, ln := range lines {
		if strings.Contains(ln, "NOTE: To recalculate") && i != len(lines)-1 {
			t.Errorf("signature text leaked into a data row at line %d:\n%s", i, result)
		}
	}
	// TOTAL counts only the two real rows (1200 + 600 = 1800).
	tl := totalRow(t, result)
	if !strings.Contains(tl, "1,800.00") {
		t.Errorf("expected MO total 1,800.00 (signature excluded), got: %s", tl)
	}
}

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

func TestUsageTextContent(t *testing.T) {
	got := stripANSI(usageText())
	if !strings.Contains(got, "retotal v1.0.0") {
		t.Errorf("usage screen missing version line 'retotal v1.0.0':\n%s", got)
	}
	if !strings.Contains(got, "Consolidate") {
		t.Errorf("usage screen should name the consolidation mode:\n%s", got)
	}
	if !strings.Contains(got, "recompute") {
		t.Errorf("usage screen should name the re-tally mode:\n%s", got)
	}
}

// TestUsageScreenExitsZero re-execs the test binary so it can observe the
// os.Exit(0) from the information-screen paths (-h, --help, wrong arg count).
func TestUsageScreenExitsZero(t *testing.T) {
	if mode := os.Getenv("RETOTAL_USAGE_MODE"); mode != "" {
		switch mode {
		case "h":
			os.Args = []string{"retotal", "-h"}
		case "help":
			os.Args = []string{"retotal", "--help"}
		case "none":
			os.Args = []string{"retotal"}
		case "two":
			os.Args = []string{"retotal", "a", "b"}
		}
		main()
		return
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test binary: %v", err)
	}
	for _, mode := range []string{"h", "help", "none", "two"} {
		cmd := exec.Command(exe, "-test.run=^TestUsageScreenExitsZero$")
		cmd.Env = append(os.Environ(), "RETOTAL_USAGE_MODE="+mode)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("mode %q: expected exit 0, got %v\noutput:\n%s", mode, err, out)
			continue
		}
		clean := stripANSI(string(out))
		if !strings.Contains(clean, "retotal v1.0.0") {
			t.Errorf("mode %q: information screen missing version line:\n%s", mode, clean)
		}
	}
}

func TestVersionAliases(t *testing.T) {
	const envName = "GKIT_RETOTAL_VERSION_FLAG"
	if flag := os.Getenv(envName); flag != "" {
		os.Args = []string{programName, flag}
		main()
		os.Exit(0)
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test binary: %v", err)
	}
	for _, flag := range []string{"-v", "--version"} {
		t.Run(flag, func(t *testing.T) {
			cmd := exec.Command(exe, "-test.run=^TestVersionAliases$")
			cmd.Env = append(os.Environ(), envName+"="+flag)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("run %s: %v", flag, err)
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
