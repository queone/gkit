package help

import (
	"strings"
	"testing"

	"github.com/queone/gkit/internal/color"
)

// No test here calls t.Parallel(): forceColor mutates package-level state in
// internal/color.

func sample() Doc {
	return Doc{
		Name:        "demo",
		Version:     "1.2.3",
		Description: "Demonstrate the shared help renderer",
		URL:         URL("demo"),
		Sections: []Section{
			{Title: "Usage", Rows: []Row{{"demo [flags] FILE", ""}}},
			{Title: "Options", Rows: []Row{{"-f, --force", "Overwrite FILE"}, {"-o, --out DIR", "Write into DIR"}}},
			{Title: "Examples", Lines: []string{"demo -f a.txt"}},
		},
	}
}

func forceColor(t *testing.T, on bool) {
	t.Helper()
	restoreEnabled := color.SetEnabled(on)
	restore256 := color.Set256(on)
	t.Cleanup(func() { restore256(); restoreEnabled() })
}

var plainSample = strings.Join([]string{
	"demo v1.2.3",
	"Demonstrate the shared help renderer",
	"github.com/queone/gkit/tree/main/cmd/demo",
	"",
	"Usage",
	"  demo [flags] FILE",
	"",
	"Options",
	"  -f, --force     Overwrite FILE",
	"  -o, --out DIR   Write into DIR",
	"  -v, --version   Print demo v1.2.3 and exit",
	"  -h, -?, --help  Show this help and exit",
	"",
	"Examples",
	"  demo -f a.txt",
	"",
}, "\n")

func TestPlainRenderingFollowsTheStandardLayout(t *testing.T) {
	forceColor(t, false)
	got := sample().String()
	if got != plainSample {
		t.Fatalf("plain help differs\n--- got\n%s\n--- want\n%s", got, plainSample)
	}
	if strings.Contains(got, "\x1b") {
		t.Fatal("plain help carries an escape sequence")
	}
}

func TestColorRenderingUsesOneSequencePerSpan(t *testing.T) {
	forceColor(t, true)
	got := sample().String()
	lines := strings.Split(got, "\n")
	checks := []struct {
		at   int
		want string
	}{
		{0, "\x1b[1;38;5;231mdemo\x1b[0m v1.2.3"},
		{1, "\x1b[38;5;245mDemonstrate the shared help renderer\x1b[0m"},
		{2, "\x1b[38;5;242mgithub.com/queone/gkit/tree/main/cmd/demo\x1b[0m"},
		{4, "\x1b[1;38;5;231mUsage\x1b[0m"},
		{7, "\x1b[1;38;5;231mOptions\x1b[0m"},
		{13, "\x1b[1;38;5;231mExamples\x1b[0m"},
	}
	for _, c := range checks {
		if lines[c.at] != c.want {
			t.Errorf("line %d = %q, want %q", c.at+1, lines[c.at], c.want)
		}
	}
	if strings.Contains(got, "\x1b[1m") {
		t.Error("colored help carries a bare bold sequence")
	}
	if color.ClearCode(got) != plainSample {
		t.Error("colored help differs from plain help once escapes are stripped")
	}
}

func TestRowsAlignMeaningsTwoPastTheLongestForm(t *testing.T) {
	got := Rows(Row{"a", "x"}, Row{"abc", "y"}, Row{"ab", ""})
	want := []string{"a    x", "abc  y", "ab"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("Rows = %q, want %q", got, want)
	}
}

func TestOptionsIsCreatedAfterUsageAndCommandsWhenAbsent(t *testing.T) {
	forceColor(t, false)
	d := Doc{Name: "demo", Version: "1.0.0", Description: "Demo", URL: URL("demo"), Sections: []Section{
		{Title: "Usage", Rows: []Row{{"demo COMMAND", ""}}},
		{Title: "Commands", Rows: []Row{{"go", "Run"}}},
		{Title: "Cheatsheet", Lines: []string{"demo go"}},
		{Title: "Examples", Lines: []string{"demo go"}},
	}}
	got := d.String()
	want := "\nCommands\n  go  Run\n\nOptions\n  -v, --version   Print demo v1.0.0 and exit\n  -h, -?, --help  Show this help and exit\n\nCheatsheet\n"
	if !strings.Contains(got, want) {
		t.Fatalf("Options not inserted after Commands:\n%s", got)
	}
	if strings.Count(got, "\nOptions\n") != 1 {
		t.Fatalf("Options appears %d times", strings.Count(got, "\nOptions\n"))
	}
}

func TestSectionLinesFollowRowsAfterOneBlankLine(t *testing.T) {
	forceColor(t, false)
	d := sample()
	d.Sections[0].Lines = []string{"FILE is any text file.", "", "Second paragraph."}
	got := d.String()
	want := "Usage\n  demo [flags] FILE\n\n  FILE is any text file.\n\n  Second paragraph.\n\nOptions\n"
	if !strings.Contains(got, want) {
		t.Fatalf("section lines misrendered:\n%s", got)
	}
}

func TestCheckAcceptsConformingDocs(t *testing.T) {
	docs := map[string]Doc{"sample": sample()}
	custom := sample()
	custom.Sections = []Section{custom.Sections[0], custom.Sections[1], {Title: "Cheatsheet", Lines: []string{"x"}}, custom.Sections[2]}
	docs["custom section"] = custom
	noOptions := sample()
	noOptions.Sections = []Section{noOptions.Sections[0], noOptions.Sections[2]}
	docs["no options"] = noOptions
	for name, d := range docs {
		if err := d.Check(); err != nil {
			t.Errorf("%s: Check = %v, want nil", name, err)
		}
	}
}

func TestCheckRejectsEachDefect(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Doc)
		want   string
	}{
		{"empty name", func(d *Doc) { d.Name = "" }, "empty name"},
		{"empty version", func(d *Doc) { d.Version = "" }, "empty version"},
		{"empty description", func(d *Doc) { d.Description = "" }, "empty description"},
		{"description period", func(d *Doc) { d.Description += "." }, "ends with a period"},
		{"description newline", func(d *Doc) { d.Description += "\nmore" }, "more than one line"},
		{"url mismatch", func(d *Doc) { d.URL = "https://" + d.URL }, "url"},
		{"no sections", func(d *Doc) { d.Sections = nil }, "first section is not Usage"},
		{"usage not first", func(d *Doc) { d.Sections[0], d.Sections[1] = d.Sections[1], d.Sections[0] }, "first section is not Usage"},
		{"overview", func(d *Doc) { d.Sections[1].Title = "Overview" }, "belongs in the README"},
		{"notes", func(d *Doc) { d.Sections[2].Title = "Notes" }, "belongs in the README"},
		{"two words", func(d *Doc) { d.Sections[1].Title = "Cheat sheet" }, "not one capitalized word"},
		{"all caps", func(d *Doc) { d.Sections[1].Title = "OPTIONS" }, "not one capitalized word"},
		{"repeat", func(d *Doc) { d.Sections[2].Title = "Options" }, "repeats"},
		{"empty section", func(d *Doc) { d.Sections[2].Lines = nil }, "is empty"},
		{"two custom", func(d *Doc) {
			d.Sections = append(d.Sections[:2], Section{Title: "Cheatsheet", Lines: []string{"x"}}, Section{Title: "Timestamps", Lines: []string{"y"}})
		}, "second utility-specific section"},
		{"examples not last", func(d *Doc) { d.Sections[1], d.Sections[2] = d.Sections[2], d.Sections[1] }, "out of order"},
		{"row without form", func(d *Doc) { d.Sections[1].Rows = append(d.Sections[1].Rows, Row{"", "x"}) }, "no form"},
		{"hand-written version row", func(d *Doc) { d.Sections[1].Rows = append(d.Sections[1].Rows, Row{"-v, --version", "x"}) }, "written by the renderer"},
		{"hand-written help row", func(d *Doc) { d.Sections[1].Rows = append(d.Sections[1].Rows, Row{"-h, --help", "x"}) }, "written by the renderer"},
	}
	for _, c := range cases {
		d := sample()
		c.mutate(&d)
		err := d.Check()
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: Check = %v, want error containing %q", c.name, err, c.want)
		}
	}
}
