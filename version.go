// Package gkit holds module-wide facts that every utility shares.
package gkit

import (
	_ "embed"
	"regexp"
	"strings"
)

//go:embed CHANGELOG.md
var changelog string

var releaseCell = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// Version returns the gkit release this build comes from, such as v0.93.0:
// the first CHANGELOG.md table row with a MAJOR.MINOR.PATCH version cell.
// Release prep writes that row before the release compiles. Version returns
// "" when no row matches.
func Version() string {
	return versionFrom(changelog)
}

func versionFrom(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		cells := strings.Split(line, "|")
		if len(cells) < 3 {
			continue
		}
		if v := strings.TrimSpace(cells[1]); releaseCell.MatchString(v) {
			return "v" + v
		}
	}
	return ""
}
