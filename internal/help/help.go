// Package help renders the help text every gkit utility prints for -h, -?,
// and --help: a three-line header (name and version, description, repository
// URL) followed by capitalized sections. Each utility describes its help as a
// Doc and prints Doc.String(), so no utility colors or aligns help by hand.
package help

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/queone/gkit/internal/color"
)

// URLPrefix is the repository path every utility's third help line starts with.
const URLPrefix = "github.com/queone/gkit/tree/main/cmd/"

// Doc is one utility's help: the header fields and its ordered sections.
type Doc struct {
	Name        string
	Version     string
	Description string
	URL         string
	Sections    []Section
}

// Section is one titled block. Rows render first as an aligned two-column
// table; Lines follow, each indented, with an empty string rendering as a
// blank line. Options receives the two standard rows from the renderer.
type Section struct {
	Title string
	Rows  []Row
	Lines []string
}

// Row pairs a flag, command, or synopsis form with its meaning.
type Row struct {
	Form    string
	Meaning string
}

// URL returns the repository URL for the named utility.
func URL(name string) string { return URLPrefix + name }

// Rows aligns each meaning two spaces past the longest form.
func Rows(rows ...Row) []string {
	width := 0
	for _, r := range rows {
		if n := utf8.RuneCountInString(r.Form); n > width {
			width = n
		}
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Meaning == "" {
			out = append(out, r.Form)
			continue
		}
		pad := width - utf8.RuneCountInString(r.Form)
		out = append(out, r.Form+strings.Repeat(" ", pad)+"  "+r.Meaning)
	}
	return out
}

// String renders the help text, colored when color is enabled.
func (d Doc) String() string {
	var b strings.Builder
	b.WriteString(color.Heading(d.Name) + " v" + d.Version + "\n")
	b.WriteString(color.Gra5(d.Description) + "\n")
	b.WriteString(color.Gra4(d.URL) + "\n")
	for _, s := range d.withStandardOptions() {
		b.WriteString("\n" + color.Heading(s.Title) + "\n")
		for _, l := range Rows(s.Rows...) {
			b.WriteString("  " + l + "\n")
		}
		if len(s.Rows) > 0 && len(s.Lines) > 0 {
			b.WriteString("\n")
		}
		for _, l := range s.Lines {
			if l == "" {
				b.WriteString("\n")
				continue
			}
			b.WriteString("  " + l + "\n")
		}
	}
	return b.String()
}

// standardRows are the two rows the renderer appends to Options.
func standardRows(name, version string) []Row {
	return []Row{
		{"-v, --version", "Print " + name + " v" + version + " and exit"},
		{"-h, -?, --help", "Show this help and exit"},
	}
}

// withStandardOptions returns the sections with the standard rows appended
// to Options, creating Options after Usage and Commands when absent.
func (d Doc) withStandardOptions() []Section {
	std := standardRows(d.Name, d.Version)
	out := make([]Section, 0, len(d.Sections)+1)
	found := false
	for _, s := range d.Sections {
		if s.Title == "Options" {
			s.Rows = append(append([]Row{}, s.Rows...), std...)
			found = true
		}
		out = append(out, s)
	}
	if found {
		return out
	}
	at := 0
	for i, s := range out {
		if s.Title == "Usage" || s.Title == "Commands" {
			at = i + 1
		}
	}
	opts := Section{Title: "Options", Rows: std}
	out = append(out[:at], append([]Section{opts}, out[at:]...)...)
	return out
}

var titleRe = regexp.MustCompile(`^[A-Z][a-z]+$`)

// rank orders the standard sections; a custom title ranks between Options and Examples.
func rank(title string) int {
	switch title {
	case "Usage":
		return 0
	case "Commands":
		return 1
	case "Options":
		return 2
	case "Examples":
		return 4
	}
	return 3
}

// Check reports the first way d breaks the shared help standard.
func (d Doc) Check() error {
	switch {
	case d.Name == "":
		return errors.New("help: empty name")
	case d.Version == "":
		return errors.New("help: empty version")
	case d.Description == "":
		return errors.New("help: empty description")
	case strings.HasSuffix(d.Description, "."):
		return errors.New("help: description ends with a period")
	case strings.Contains(d.Description, "\n"):
		return errors.New("help: description spans more than one line")
	case d.URL != URL(d.Name):
		return fmt.Errorf("help: url %q is not %q", d.URL, URL(d.Name))
	case len(d.Sections) == 0 || d.Sections[0].Title != "Usage":
		return errors.New("help: first section is not Usage")
	}
	last, custom := -1, 0
	seen := map[string]bool{}
	for _, s := range d.Sections {
		switch {
		case s.Title == "Overview" || s.Title == "Notes":
			return fmt.Errorf("help: section %s belongs in the README", s.Title)
		case !titleRe.MatchString(s.Title):
			return fmt.Errorf("help: title %q is not one capitalized word", s.Title)
		case seen[s.Title]:
			return fmt.Errorf("help: section %s repeats", s.Title)
		case len(s.Rows) == 0 && len(s.Lines) == 0:
			return fmt.Errorf("help: section %s is empty", s.Title)
		}
		seen[s.Title] = true
		r := rank(s.Title)
		if r == 3 {
			if custom++; custom > 1 {
				return fmt.Errorf("help: second utility-specific section %s", s.Title)
			}
		}
		if r <= last {
			return fmt.Errorf("help: section %s is out of order", s.Title)
		}
		last = r
		for _, row := range s.Rows {
			if row.Form == "" {
				return fmt.Errorf("help: section %s has a row with no form", s.Title)
			}
			if s.Title == "Options" && (strings.HasPrefix(row.Form, "-v,") || strings.HasPrefix(row.Form, "-h,")) {
				return fmt.Errorf("help: Options row %q is written by the renderer", row.Form)
			}
		}
	}
	return nil
}
