package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/queone/gkit/internal/color"
	"github.com/queone/gkit/internal/lockbox"
)

func TestAddBindsToThisMacUnlessTold(t *testing.T) {
	h := newHarness(t)
	h.mustRun("init", "-N")
	a := h.write(".a", "a\n", 0o644)
	b := h.write(".b", "b\n", 0o644)
	g := h.write(".g", "g\n", 0o644)
	if out := h.mustRun("add", a); out != line("added", "~/.a (mode 0644, host a)") {
		t.Fatalf("default binding: %q", out)
	}
	if out := h.mustRun("add", b, "-H", "b"); out != line("added", "~/.b (mode 0644, host b)") {
		t.Fatalf("-H: %q", out)
	}
	if out := h.mustRun("add", g, "-g"); out != line("added", "~/.g (mode 0644, global)") {
		t.Fatalf("-g: %q", out)
	}
	if code, _, _ := h.run("add", g, "-H", "b", "-g"); code != 2 {
		t.Fatalf("-H with -g: code %d", code)
	}
	out := h.mustRun("ls")
	for _, want := range []string{"\na ", "\nb ", "\n<global>"} {
		if !strings.Contains(out, want) {
			t.Fatalf("ls lacks %q: %q", want, out)
		}
	}
	h.write(".a", "global\n", 0o644)
	h.mustRun("add", a, "-g")
	os.Remove(a)
	h.mustRun("pull", "-f", "~/.a")
	if got := h.read(".a"); got != "a\n" {
		t.Fatalf("this Mac's entry must win over the global one, pulled %q", got)
	}
	h.app.host = ""
	n := h.write(".n", "n\n", 0o644)
	if _, errs := h.mustFail(1, "add", n); !strings.Contains(errs, "hostname is unknown") {
		t.Fatalf("nameless Mac: %q", errs)
	}
	if out := h.mustRun("add", n, "-g"); out != line("added", "~/.n (mode 0644, global)") {
		t.Fatalf("nameless Mac with -g: %q", out)
	}
}

func TestSetRebindsAndChangesMode(t *testing.T) {
	h := newHarness(t)
	h.app.host = "np11"
	h.mustRun("init", "-N")
	cfg := h.write(".ssh/config", "global\n", 0o644)
	h.mustRun("add", cfg, "-g")
	h.write(".ssh/config", "np10\n", 0o644)
	h.mustRun("add", cfg, "-H", "np10")
	st := h.openStore()
	before, _ := st.Entries()
	st.Close()
	globalID := before[0].ID
	if before[0].Host != "" || before[1].Host != "np10" {
		t.Fatalf("fixture: %+v", before)
	}

	if out := h.mustRun("set", cfg, "-H", "np11"); out != line("set", "~/.ssh/config (host np11, mode 0644)") {
		t.Fatalf("rebind global to np11: %q", out)
	}
	st = h.openStore()
	after, _ := st.Entries()
	for _, e := range after {
		if e.ID == globalID && (e.Host != "np11" || e.Mode != 0o644) {
			t.Fatalf("rebound entry: %+v", e)
		}
		if e.ID != globalID && e.Host != "np10" {
			t.Fatalf("np10 entry disturbed: %+v", e)
		}
		if n, _ := st.VersionCount(e.ID); n != 1 {
			t.Fatalf("versions lost on %+v: %d", e, n)
		}
	}
	st.Close()
	if out := h.mustRun("cat", cfg); out != "global\n" {
		t.Fatalf("np11 now reads the rebound entry: %q", out)
	}

	if out := h.mustRun("set", cfg, "-g", "-F", "np10"); out != line("set", "~/.ssh/config (global, mode 0644)") {
		t.Fatalf("np10 to global: %q", out)
	}
	out := h.mustRun("ls")
	if !strings.Contains(out, "\nnp11 ") || !strings.Contains(out, "\n<global>") || strings.Contains(out, "\nnp10 ") {
		t.Fatalf("ls after rebinding: %q", out)
	}

	if out := h.mustRun("set", cfg, "-m", "600"); out != line("set", "~/.ssh/config (host np11, mode 0600)") {
		t.Fatalf("mode change: %q", out)
	}
	if _, errs := h.mustFail(1, "set", cfg, "-F", "global", "-H", "np11"); !strings.Contains(errs, "already has an entry that is host np11") {
		t.Fatalf("conflicting rebind: %q", errs)
	}
	for _, args := range [][]string{{"set", cfg}, {"set", cfg, "-H", "x", "-g"}, {"set", cfg, "-m", "99"}, {"set", cfg, "-m", "08"}} {
		if code, _, _ := h.run(args...); code != 2 {
			t.Fatalf("%v: code %d, want 2", args, code)
		}
	}
	if _, errs := h.mustFail(1, "set", "~/nothing", "-m", "600"); !strings.Contains(errs, "not registered") {
		t.Fatalf("unknown target: %q", errs)
	}
	if _, errs := h.mustFail(1, "set", cfg, "-F", "np12", "-g"); !strings.Contains(errs, "not registered as host np12") {
		t.Fatalf("unknown -F: %q", errs)
	}
}

func TestLsLayoutOwnershipAndColor(t *testing.T) {
	h := newHarness(t)
	h.mustRun("init", "-N")
	live := h.write(".bashrc", "x\n", 0o644)
	h.mustRun("add", live)
	other := h.write(".profile", "y\n", 0o600)
	h.mustRun("add", other, "-g")
	st := h.openStore()
	e, err := st.AddEntry("/etc/hosts", 0o644, "", lockbox.UnknownOwnership)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddVersion(e.ID, []byte("h\n"), "a"); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
	st.Close()

	out := h.mustRun("ls")
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	cols := regexp.MustCompile(`\s{2,}`)
	if got := strings.Join(cols.Split(lines[0], -1), " "); got != "HOST OWNER_GROUP MODE CAPTURED TARGET" {
		t.Fatalf("header %q", lines[0])
	}
	info, err := os.Stat(live)
	if err != nil {
		t.Fatal(err)
	}
	own := fileOwnership(info)
	if !own.Known() || own.Owner == "" || own.Group == "" {
		t.Fatalf("ownership of the live file: %+v", own)
	}
	rows := map[string][]string{}
	for _, l := range lines[1:] {
		f := cols.Split(l, -1)
		if len(f) != 5 {
			t.Fatalf("row %q", l)
		}
		rows[f[4]] = f
	}
	if r := rows["~/.bashrc"]; r[0] != "a" || r[1] != own.Owner+":"+own.Group || r[2] != "0644" {
		t.Fatalf("this Mac's row: %v", r)
	}
	if r := rows["~/.profile"]; r[0] != "<global>" || r[2] != "0600" {
		t.Fatalf("global row: %v", r)
	}
	if r := rows["/etc/hosts"]; r[0] != "<global>" || r[1] != "?:?" {
		t.Fatalf("unknown ownership row: %v", r)
	}
	if !strings.HasPrefix(lines[1], "<global>") && !strings.HasPrefix(lines[1], "a ") {
		t.Fatalf("host is not the first column: %q", lines[1])
	}

	defer color.SetEnabled(true)()
	_, colored, _ := h.run("ls")
	if color.ClearCode(colored) != out {
		t.Fatal("colored ls does not strip to the plain listing")
	}
	clines := strings.Split(strings.TrimSuffix(colored, "\n"), "\n")
	if strings.Contains(clines[0], "\x1b[") {
		t.Fatalf("header is colored: %q", clines[0])
	}
	for _, l := range clines[1:] {
		if !strings.HasPrefix(l, "\x1b[38;5;242m") || strings.Count(l, "\x1b[38;5;242m") != 5 {
			t.Fatalf("row values not all dark grey: %q", l)
		}
	}
}

func TestPushRefreshesOwnership(t *testing.T) {
	h := newHarness(t)
	h.mustRun("init", "-N")
	live := h.write(".bashrc", "x\n", 0o644)
	h.mustRun("add", live)
	st := h.openStore()
	entries, _ := st.Entries()
	if err := st.SetOwner(entries[0].ID, lockbox.Ownership{UID: 1, GID: 2, Owner: "u", Group: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
	st.Close()
	if out := h.mustRun("push"); out != line("updated", "~/.bashrc") {
		t.Fatalf("push with stale ownership: %q", out)
	}
	info, _ := os.Stat(live)
	st = h.openStore()
	entries, _ = st.Entries()
	st.Close()
	if entries[0].Ownership != fileOwnership(info) {
		t.Fatalf("ownership after push %+v, want %+v", entries[0].Ownership, fileOwnership(info))
	}
	if out := h.mustRun("push"); out != line("unchanged", "~/.bashrc") {
		t.Fatalf("second push: %q", out)
	}
}

func TestParseMode(t *testing.T) {
	for in, want := range map[string]os.FileMode{"600": 0o600, "0644": 0o644, "755": 0o755} {
		if got, err := parseMode(in); err != nil || got != want {
			t.Fatalf("parseMode(%s) = %o, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "6", "66", "08", "1777", "0x1", "rw-"} {
		if _, err := parseMode(bad); err == nil {
			t.Fatalf("parseMode(%q) accepted", bad)
		}
	}
}
