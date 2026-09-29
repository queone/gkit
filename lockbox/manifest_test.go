package lockbox

import (
	"errors"
	"path/filepath"
	"testing"
)

func testEnv(vars map[string]string, home string) Env {
	return envFrom(func(k string) string { return vars[k] }, home)
}

func TestTemplateUsesLongestBaseDirectoryAndExpandsBack(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	cfg := filepath.Join(home, ".config")
	e := testEnv(map[string]string{"XDG_CONFIG_HOME": cfg, "CLAUDE_CONFIG_DIR": filepath.Join(cfg, "claude")}, home)
	cases := map[string]string{
		filepath.Join(cfg, "git", "config"):              "$XDG_CONFIG_HOME/git/config",
		filepath.Join(cfg, "claude", "settings.json"):    "$CLAUDE_CONFIG_DIR/settings.json",
		filepath.Join(home, ".bashrc"):                   "~/.bashrc",
		filepath.Join(home, ".local", "share", "x", "y"): "$XDG_DATA_HOME/x/y",
		"/etc/hosts": "/etc/hosts",
	}
	for path, want := range cases {
		got := e.Template(path)
		if got != want {
			t.Errorf("Template(%s) = %s, want %s", path, got, want)
		}
		if back := e.Expand(got); back != path {
			t.Errorf("Expand(%s) = %s, want %s", got, back, path)
		}
	}
	if got := e.Literal(filepath.Join(cfg, "git", "config")); got != "~/.config/git/config" {
		t.Errorf("Literal = %s", got)
	}
}

func TestExpandAppliesXDGFallbacksWhenUnset(t *testing.T) {
	home := "/srv/example"
	e := testEnv(map[string]string{}, home)
	cases := map[string]string{
		"$XDG_CONFIG_HOME/git/config": home + "/.config/git/config",
		"$XDG_DATA_HOME/a":            home + "/.local/share/a",
		"$XDG_STATE_HOME/b":           home + "/.local/state/b",
		"$XDG_CACHE_HOME/c":           home + "/.cache/c",
		"$CLAUDE_CONFIG_DIR/d":        home + "/.claude/d",
		"~/.bashrc":                   home + "/.bashrc",
		"$HOME/.profile":              home + "/.profile",
		"/etc/hosts":                  "/etc/hosts",
	}
	for target, want := range cases {
		if got := e.Expand(target); got != want {
			t.Errorf("Expand(%s) = %s, want %s", target, got, want)
		}
	}
	rel := testEnv(map[string]string{"XDG_CONFIG_HOME": "relative/dir"}, home)
	if got := rel.Expand("$XDG_CONFIG_HOME/x"); got != home+"/.config/x" {
		t.Errorf("relative XDG value must fall back, got %s", got)
	}
}

func TestSelectPrefersHostBoundEntries(t *testing.T) {
	entries := []Entry{
		{ID: 1, Target: "~/.bashrc", Host: ""},
		{ID: 2, Target: "~/.bashrc", Host: "a"},
		{ID: 3, Target: "~/.bashrc", Host: "b"},
		{ID: 4, Target: "~/.vimrc", Host: "b"},
		{ID: 5, Target: "~/.profile", Host: ""},
	}
	got := Select(entries, "a")
	if len(got) != 2 || got[0].ID != 2 || got[1].ID != 5 {
		t.Fatalf("host a: %+v", got)
	}
	got = Select(entries, "c")
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 5 {
		t.Fatalf("host c: %+v", got)
	}
	got = Select(entries, "b")
	if len(got) != 3 || got[0].ID != 3 || got[2].ID != 4 {
		t.Fatalf("host b: %+v", got)
	}
}

func TestHostnameFromScutilWithFallback(t *testing.T) {
	scutil := func(name string, args ...string) ([]byte, error) {
		if name == "scutil" && len(args) == 2 && args[0] == "--get" && args[1] == "LocalHostName" {
			return []byte("np10\n"), nil
		}
		return nil, errors.New("unexpected command")
	}
	if got := Hostname(scutil, func() (string, error) { return "other.local", nil }); got != "np10" {
		t.Fatalf("scutil path: %q", got)
	}
	failing := func(string, ...string) ([]byte, error) { return nil, errors.New("no scutil") }
	if got := Hostname(failing, func() (string, error) { return "np10.local", nil }); got != "np10" {
		t.Fatalf("fallback path: %q", got)
	}
	if got := Hostname(nil, func() (string, error) { return "", errors.New("none") }); got != "" {
		t.Fatalf("no hostname: %q", got)
	}
}
