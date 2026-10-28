package gkit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// checkReadme reports why the README at path breaks the utility README rules:
// the file must exist and hold exactly one "### Why" heading outside code
// fences, followed by text before the next heading.
func checkReadme(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var whys int
	var inFence, wantText bool
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		heading := strings.HasPrefix(line, "#")
		if wantText && strings.TrimSpace(line) != "" {
			if heading {
				return errors.New("### Why section has no text")
			}
			wantText = false
		}
		if line == "### Why" {
			whys++
			wantText = true
		}
	}
	switch {
	case whys == 0:
		return errors.New("no ### Why section")
	case whys > 1:
		return fmt.Errorf("%d ### Why sections, want 1", whys)
	case wantText:
		return errors.New("### Why section has no text")
	}
	return nil
}

func TestEveryUtilityHasAReadmeWithWhy(t *testing.T) {
	dirs, err := filepath.Glob("cmd/*")
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) == 0 {
		t.Fatal("found no cmd/* directory")
	}
	for _, dir := range dirs {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			continue
		}
		if err := checkReadme(filepath.Join(dir, "README.md")); err != nil {
			t.Errorf("%s/README.md: %v", dir, err)
		}
	}
}

func TestCheckReadmeRules(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name, text string
		ok         bool
	}{
		{"valid", "## x\nDoes x.\n\n### Why\nBecause.\n\n### Usage\n", true},
		{"fenced hashes", "## x\n```text\n# not a heading\n### Why\n```\n### Why\n\n# Because.\n", false},
		{"fenced hashes only", "## x\n```text\n# GitHub, every name\n```\n### Why\nBecause.\n", true},
		{"question mark", "## x\n### Why?\nBecause.\n", false},
		{"no text", "## x\n### Why\n\n### Usage\n", false},
		{"no text at end", "## x\n### Why\n\n", false},
		{"two whys", "### Why\nA.\n### Why\nB.\n", false},
	} {
		path := filepath.Join(dir, tc.name+".md")
		if err := os.WriteFile(path, []byte(tc.text), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := checkReadme(path); (err == nil) != tc.ok {
			t.Errorf("%s: err %v, want ok=%v", tc.name, err, tc.ok)
		}
	}
	if err := checkReadme(filepath.Join(dir, "missing.md")); err == nil {
		t.Error("missing README: want an error")
	}
}
