// Package notice prints a one-line stderr notice when a newer gkit release
// exists. Every gkit utility calls Show first in main.
package notice

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"

	"github.com/mattn/go-isatty"

	"github.com/queone/gkit"
	"github.com/queone/gkit/update"
)

const (
	module      = "github.com/queone/gkit"
	cloneHint   = "run git pull and ./build.sh in your gkit clone"
	installHint = "run go install github.com/queone/gkit/cmd/...@latest"
)

// release matches the main-module version of a go install build.
var release = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// Show checks, at most once a day, for a newer gkit release and prints the
// notice to stderr when one exists. It does nothing when stderr is not a
// terminal, and prints nothing for an error, a skip, or no update.
func Show() {
	fd := os.Stderr.Fd()
	if !isatty.IsTerminal(fd) && !isatty.IsCygwinTerminal(fd) {
		return
	}
	o, err := defaultOptions()
	if err != nil {
		return
	}
	var mainVersion string
	if bi, ok := debug.ReadBuildInfo(); ok {
		mainVersion = bi.Main.Version
	}
	show(os.Stderr, true, mainVersion, o)
}

// defaultOptions returns the check every utility shares: the gkit module, the
// built-in gkit version, and one cache file for all utilities.
func defaultOptions() (update.Options, error) {
	dir, err := update.CacheDir("gkit")
	if err != nil {
		return update.Options{}, err
	}
	return update.Options{
		Module:    module,
		Current:   gkit.Version(),
		CacheFile: filepath.Join(dir, "update.json"),
	}, nil
}

// show runs the check and prints the notice to w. A strict mainVersion marks
// a go install build; anything else marks a build from a clone.
func show(w io.Writer, terminal bool, mainVersion string, o update.Options) {
	if !terminal {
		return
	}
	r, err := update.Check(o)
	if err != nil || !r.Available {
		return
	}
	hint := cloneHint
	if release.MatchString(mainVersion) {
		hint = installHint
	}
	fmt.Fprintf(w, "gkit %s is out (this build is %s): %s\n", r.Latest, r.Current, hint)
}
