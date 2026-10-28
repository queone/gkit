package gkit

import (
	"os"
	"strings"
	"testing"
)

func TestVersionFromChangelog(t *testing.T) {
	for _, tc := range []struct{ name, text, want string }{
		{"unreleased first", "| Version | Summary |\n|---------|---------|\n| Unreleased | |\n| 0.93.0 | x |\n| 0.92.0 | y |\n", "v0.93.0"},
		{"no release row", "| Version | Summary |\n|---------|---------|\n| Unreleased | |\n", ""},
		{"malformed cell", "| Unreleased | |\n| 0.93 | x |\n| v0.92.1 | y |\n| 01.2.3 | z |\n| 0.92.0 | w |\n", "v0.92.0"},
		{"empty", "", ""},
	} {
		if got := versionFrom(tc.text); got != tc.want {
			t.Errorf("%s: versionFrom = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestVersionReadsTheEmbeddedChangelog(t *testing.T) {
	data, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	got := Version()
	if got == "" || got != versionFrom(string(data)) {
		t.Fatalf("Version() = %q, want the first release row of CHANGELOG.md", got)
	}
	if !strings.HasPrefix(got, "v") {
		t.Fatalf("Version() = %q, want a v prefix", got)
	}
}
