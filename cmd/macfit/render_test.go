package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/queone/gkit/internal/lockbox"
)

// renderFixture registers an unbound file, an unbound XDG file, a literal
// absolute target, and a host-bound file. It returns the live paths.
func renderFixture(h *harness) (bashrc, gitcfg string) {
	h.mustRun("init", "-N")
	bashrc = h.write(".bashrc", "shared\n", 0o644)
	gitcfg = h.write(".config/git/config", "[user]\n\tname = x\n", 0o600)
	h.mustRun("add", bashrc, gitcfg, "-g")
	local := h.write(".bashrc.local", "for np11\n", 0o600)
	h.mustRun("add", local, "-H", "np11")
	st := h.openStore()
	e, err := st.AddEntry("/etc/hosts", 0o644, "", lockbox.UnknownOwnership)
	if err != nil {
		h.t.Fatal(err)
	}
	if _, err := st.AddVersion(e.ID, []byte("127.0.0.1 localhost\n"), "a"); err != nil {
		h.t.Fatal(err)
	}
	if err := st.Save(); err != nil {
		h.t.Fatal(err)
	}
	st.Close()
	return bashrc, gitcfg
}

// renderedDir parses the summary line and registers the directory for cleanup.
func renderedDir(t *testing.T, out string) string {
	t.Helper()
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	last := lines[len(lines)-1]
	_, after, ok := strings.Cut(last, " into ")
	if !strings.HasPrefix(last, "rendered ") || !ok {
		t.Fatalf("no summary line: %q", out)
	}
	dir := after
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func modeOf(t *testing.T, p string) fs.FileMode {
	t.Helper()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func TestRenderDefaultTreeAndManifest(t *testing.T) {
	h := newHarness(t)
	renderFixture(h)
	out := h.mustRun("render")
	dir := renderedDir(t, out)
	if !strings.HasPrefix(filepath.Base(dir), "macfit-render-") || modeOf(t, dir) != 0o700 {
		t.Fatalf("render dir %s mode %o", dir, modeOf(t, dir))
	}
	if !strings.HasSuffix(out, "rendered 4 file(s) into "+dir+"\n") {
		t.Fatalf("summary: %q", out)
	}
	want := map[string]struct {
		content string
		mode    fs.FileMode
	}{
		"any/HOME/.bashrc":               {"shared\n", 0o644},
		"any/XDG_CONFIG_HOME/git/config": {"[user]\n\tname = x\n", 0o600},
		"any/root/etc/hosts":             {"127.0.0.1 localhost\n", 0o644},
		"np11/HOME/.bashrc.local":        {"for np11\n", 0o600},
	}
	for rel, w := range want {
		if !strings.Contains(out, line("rendered", rel)) {
			t.Fatalf("no rendered line for %s: %q", rel, out)
		}
		b, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != w.content || modeOf(t, filepath.Join(dir, rel)) != w.mode {
			t.Fatalf("%s: content %q mode %o", rel, b, modeOf(t, filepath.Join(dir, rel)))
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "versions")); !os.IsNotExist(err) {
		t.Fatal("versions subtree rendered without -a")
	}
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && modeOf(t, p) != 0o700 {
			t.Fatalf("directory %s has mode %o", p, modeOf(t, p))
		}
		return nil
	})

	m, err := os.ReadFile(filepath.Join(dir, "MANIFEST.txt"))
	if err != nil {
		t.Fatal(err)
	}
	mlines := strings.Split(strings.TrimSuffix(string(m), "\n"), "\n")
	head := []string{"store: " + h.store, fmt.Sprintf("generation: %d", h.generation()), "rendered: ", "note: plaintext copies of the store; delete this directory when done", ""}
	for i, w := range head {
		if !strings.HasPrefix(mlines[i], w) {
			t.Fatalf("manifest line %d %q, want prefix %q", i, mlines[i], w)
		}
	}
	rows := mlines[len(head):]
	if len(rows) != 4 {
		t.Fatalf("manifest rows %d: %v", len(rows), rows)
	}
	for _, r := range rows {
		f := strings.Split(r, "\t")
		if len(f) != 7 {
			t.Fatalf("manifest row %q", r)
		}
		b, _ := os.ReadFile(filepath.Join(dir, f[0]))
		if lockbox.Digest(b) != f[6] {
			t.Fatalf("manifest digest mismatch for %s", f[0])
		}
		if f[0] == "np11/HOME/.bashrc.local" && (f[2] != "np11" || f[3] != "0600") {
			t.Fatalf("manifest host/mode row %q", r)
		}
		if f[0] == "any/HOME/.bashrc" && f[2] != "-" {
			t.Fatalf("manifest unbound host %q", r)
		}
	}
}

func TestRenderOutDirRules(t *testing.T) {
	h := newHarness(t)
	renderFixture(h)
	out := filepath.Join(h.root, "out")
	h.mustRun("render", "-o", out)
	if modeOf(t, out) != 0o700 {
		t.Fatalf("created out dir mode %o", modeOf(t, out))
	}
	extra := filepath.Join(out, "keep.txt")
	if err := os.WriteFile(extra, []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := md5Tree(t, out)
	stdout, errs := h.mustFail(1, "render", "-o", out)
	if strings.Contains(stdout, "rendered") || !strings.Contains(errs, "not empty") {
		t.Fatalf("non-empty refusal: out %q err %q", stdout, errs)
	}
	if md5Tree(t, out) != before {
		t.Fatal("refused render changed the directory")
	}
	h.mustRun("render", "-o", out, "-f")
	if b, _ := os.ReadFile(extra); string(b) != "mine" {
		t.Fatal("-f removed an unrelated file")
	}
	if _, err := os.Stat(filepath.Join(out, "any", "HOME", ".bashrc")); err != nil {
		t.Fatal("-f did not write the tree")
	}
	if code, _, _ := h.run("render", "extra"); code != 2 {
		t.Fatalf("render with a positional: code %d", code)
	}
}

func TestRenderAllVersions(t *testing.T) {
	h := newHarness(t)
	h.mustRun("init", "-N")
	live := h.write(".bashrc", "one\n", 0o644)
	h.mustRun("add", live, "-g")
	h.write(".bashrc", "two\n", 0o644)
	h.mustRun("push")
	out := h.mustRun("render", "-a")
	dir := renderedDir(t, out)
	if b, _ := os.ReadFile(filepath.Join(dir, "any", "HOME", ".bashrc")); string(b) != "two\n" {
		t.Fatalf("latest tree holds %q", b)
	}
	vdir := filepath.Join(dir, "versions", "any", "HOME", ".bashrc")
	names, err := os.ReadDir(vdir)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("version files: %d", len(names))
	}
	for i, want := range []string{"one\n", "two\n"} {
		n := names[i].Name()
		if !strings.HasPrefix(n, fmt.Sprintf("%d-", i+1)) || len(n) != len("1-20060102T150405Z") {
			t.Fatalf("version file name %q", n)
		}
		if b, _ := os.ReadFile(filepath.Join(vdir, n)); string(b) != want {
			t.Fatalf("version %s holds %q", n, b)
		}
	}
}

func TestRenderAndCatAreReadOnly(t *testing.T) {
	h := newHarness(t)
	bashrc, gitcfg := renderFixture(h)
	storeBefore, _ := os.ReadFile(h.store)
	liveBefore := md5Tree(t, h.home)
	gen := h.generation()
	out := h.mustRun("render", "-a")
	renderedDir(t, out)
	h.mustRun("cat", bashrc)
	h.mustRun("cat", gitcfg)
	storeAfter, _ := os.ReadFile(h.store)
	if !bytes.Equal(storeBefore, storeAfter) || h.generation() != gen {
		t.Fatal("render or cat changed the store")
	}
	if md5Tree(t, h.home) != liveBefore {
		t.Fatal("render or cat changed a live file")
	}
}

func TestCatPrintsExactBytesAndSelectsHosts(t *testing.T) {
	h := newHarness(t)
	h.mustRun("init", "-N")
	noNL := h.write(".hushlogin", "no newline at end", 0o644)
	empty := h.write(".empty", "", 0o644)
	shared := h.write(".bashrc", "shared\n", 0o644)
	h.mustRun("add", noNL, empty, shared, "-g")
	h.write(".bashrc", "for a\n", 0o644)
	h.mustRun("add", shared, "-H", "a")
	h.write(".bashrc", "for b\n", 0o644)
	h.mustRun("add", shared, "-H", "b")

	if out := h.mustRun("cat", noNL); out != "no newline at end" {
		t.Fatalf("cat without trailing newline: %q", out)
	}
	if out := h.mustRun("cat", "~/.empty"); out != "" {
		t.Fatalf("cat empty: %q", out)
	}
	if out := h.mustRun("cat", shared); out != "for a\n" {
		t.Fatalf("cat on host a: %q", out)
	}
	if out := h.mustRun("cat", shared, "-H", "b"); out != "for b\n" {
		t.Fatalf("cat -H b: %q", out)
	}
	h.app.host = "c"
	if out := h.mustRun("cat", shared); out != "shared\n" {
		t.Fatalf("cat on host c: %q", out)
	}
	if _, errs := h.mustFail(1, "cat", "~/nothing"); !strings.Contains(errs, "not registered") {
		t.Fatalf("cat unknown: %q", errs)
	}
	if code, _, _ := h.run("cat"); code != 2 {
		t.Fatalf("cat without target: code %d", code)
	}
}

func TestRenderPathMapping(t *testing.T) {
	cases := map[string]string{
		"~/.bashrc":                   "HOME/.bashrc",
		"~":                           "HOME",
		"$XDG_CONFIG_HOME/git/config": "XDG_CONFIG_HOME/git/config",
		"$CLAUDE_CONFIG_DIR/x":        "CLAUDE_CONFIG_DIR/x",
		"/etc/hosts":                  "root/etc/hosts",
	}
	for target, want := range cases {
		if got := renderPath(target); got != want {
			t.Errorf("renderPath(%s) = %s, want %s", target, got, want)
		}
	}
	if scopeOf("") != "any" || scopeOf("np11") != "np11" {
		t.Fatal("scopeOf")
	}
}

// md5Tree digests every regular file under root by path and content.
func md5Tree(t *testing.T, root string) string {
	t.Helper()
	hash := md5.New()
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		fmt.Fprintf(hash, "%s\n", p)
		hash.Write(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}
