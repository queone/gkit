package lockbox

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Env holds the base directories used to template live paths into store
// targets and to expand targets back into live paths.
type Env struct {
	Home       string
	ConfigHome string
	DataHome   string
	StateHome  string
	CacheHome  string
	ClaudeDir  string
}

// EnvFromOS reads the base directories from the environment with the XDG
// fallbacks applied for unset variables.
func EnvFromOS() Env {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.Getenv("HOME")
	}
	return envFrom(os.Getenv, home)
}

func envFrom(get func(string) string, home string) Env {
	pick := func(name, fallback string) string {
		if v := get(name); v != "" && filepath.IsAbs(v) {
			return filepath.Clean(v)
		}
		return filepath.Join(home, fallback)
	}
	return Env{
		Home:       filepath.Clean(home),
		ConfigHome: pick("XDG_CONFIG_HOME", ".config"),
		DataHome:   pick("XDG_DATA_HOME", ".local/share"),
		StateHome:  pick("XDG_STATE_HOME", ".local/state"),
		CacheHome:  pick("XDG_CACHE_HOME", ".cache"),
		ClaudeDir:  pick("CLAUDE_CONFIG_DIR", ".claude"),
	}
}

type prefix struct{ token, dir string }

func (e Env) prefixes() []prefix {
	ps := []prefix{
		{"$CLAUDE_CONFIG_DIR", e.ClaudeDir},
		{"$XDG_CONFIG_HOME", e.ConfigHome},
		{"$XDG_DATA_HOME", e.DataHome},
		{"$XDG_STATE_HOME", e.StateHome},
		{"$XDG_CACHE_HOME", e.CacheHome},
		{"~", e.Home},
	}
	sort.SliceStable(ps, func(i, j int) bool { return len(ps[i].dir) > len(ps[j].dir) })
	return ps
}

// Template converts an absolute live path into a store target by substituting
// the longest matching base directory. A path outside every base directory is
// returned unchanged.
func (e Env) Template(path string) string {
	path = filepath.Clean(path)
	for _, p := range e.prefixes() {
		if p.dir == "" {
			continue
		}
		if path == p.dir {
			return p.token
		}
		if strings.HasPrefix(path, p.dir+"/") {
			return p.token + path[len(p.dir):]
		}
	}
	return path
}

// Literal converts an absolute live path into a store target using only the
// home directory, so XDG locations stay spelled out.
func (e Env) Literal(path string) string {
	return Env{Home: e.Home}.Template(path)
}

// Expand converts a store target into a live path on this machine.
func (e Env) Expand(target string) string {
	for _, p := range e.prefixes() {
		if target == p.token {
			return p.dir
		}
		if strings.HasPrefix(target, p.token+"/") {
			return filepath.Join(p.dir, target[len(p.token)+1:])
		}
	}
	if target == "$HOME" {
		return e.Home
	}
	if strings.HasPrefix(target, "$HOME/") {
		return filepath.Join(e.Home, target[len("$HOME/"):])
	}
	return target
}

// Hostname returns the Mac's LocalHostName through exec, falling back to the
// operating-system hostname with any .local suffix removed.
func Hostname(exec Executor, osHostname func() (string, error)) string {
	if exec != nil {
		if out, err := exec("scutil", "--get", "LocalHostName"); err == nil {
			if h := strings.TrimSpace(string(out)); h != "" {
				return h
			}
		}
	}
	h, err := osHostname()
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(strings.TrimSpace(h), ".local")
}

// Ownership is the owner and group of a live file as captured on the Mac
// that added or last pushed it. Unknown ownership has UID and GID -1 and
// empty names.
type Ownership struct {
	UID   int64
	GID   int64
	Owner string
	Group string
}

// UnknownOwnership is the ownership of an entry that predates schema
// version 2 or whose file could not be inspected.
var UnknownOwnership = Ownership{UID: -1, GID: -1}

// Known reports whether the ownership carries real values.
func (o Ownership) Known() bool { return o.UID >= 0 && o.GID >= 0 }

// Entry is one registered file: its target template, mode, optional host
// binding, and ownership.
type Entry struct {
	ID     int64
	Target string
	Mode   os.FileMode
	Host   string
	Ownership
}

// Select returns the entries that apply on host, one per target. An entry
// bound to host wins over an unbound entry for the same target; entries bound
// to other hosts are skipped.
func Select(entries []Entry, host string) []Entry {
	chosen := map[string]Entry{}
	for _, e := range entries {
		switch {
		case e.Host != "" && e.Host == host:
			chosen[e.Target] = e
		case e.Host == "":
			if cur, ok := chosen[e.Target]; !ok || cur.Host == "" {
				chosen[e.Target] = e
			}
		}
	}
	out := make([]Entry, 0, len(chosen))
	for _, e := range chosen {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Target < out[j].Target })
	return out
}
