package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/queone/gkit/internal/color"
	"github.com/queone/gkit/internal/lockbox"
)

// testKDF keeps passphrase derivation fast in tests.
var testKDF = lockbox.KDF{Time: 1, Memory: 8 * 1024, Threads: 1}

// line renders the plain, padded status line the CLI prints for a target.
func line(word, target string) string {
	return fmt.Sprintf("%-*s%s\n", statusWidth, word, target)
}

type harness struct {
	t     *testing.T
	app   *app
	out   *bytes.Buffer
	errb  *bytes.Buffer
	root  string
	home  string
	store string
	keys  *lockbox.MemoryKeyStore
}

func testEnv(home string) lockbox.Env {
	return lockbox.Env{
		Home:       home,
		ConfigHome: filepath.Join(home, ".config"),
		DataHome:   filepath.Join(home, ".local", "share"),
		StateHome:  filepath.Join(home, ".local", "state"),
		CacheHome:  filepath.Join(home, ".cache"),
		ClaudeDir:  filepath.Join(home, ".config", "claude"),
	}
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(filepath.Join(home, ".config", "git"), 0o755); err != nil {
		t.Fatal(err)
	}
	synced := filepath.Join(root, "synced")
	if err := os.MkdirAll(synced, 0o755); err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, out: &bytes.Buffer{}, errb: &bytes.Buffer{}, root: root, home: home,
		store: filepath.Join(synced, "macfit.store"), keys: &lockbox.MemoryKeyStore{}}
	h.app = &app{
		stdout: h.out, stderr: h.errb, keys: h.keys, env: testEnv(home), host: "a", kdf: testKDF, goos: "darwin",
		isTerminal: func() bool { return true },
		readSecret: func(string) ([]byte, error) { return []byte("pw"), nil },
		readLine:   func(string) (string, error) { return "", nil },
	}
	return h
}

// run executes macfit against the harness store through -s.
func (h *harness) run(args ...string) (int, string, string) {
	return h.runRaw(append([]string{"-s", h.store}, args...)...)
}

// runRaw executes macfit with the arguments as given, no -s added.
func (h *harness) runRaw(args ...string) (int, string, string) {
	h.out.Reset()
	h.errb.Reset()
	code := h.app.run(args)
	return code, h.out.String(), h.errb.String()
}

func (h *harness) mustRun(args ...string) string {
	h.t.Helper()
	code, out, errs := h.run(args...)
	if code != 0 {
		h.t.Fatalf("macfit %s: exit %d\nstdout: %s\nstderr: %s", strings.Join(args, " "), code, out, errs)
	}
	return out
}

func (h *harness) mustFail(want int, args ...string) (string, string) {
	h.t.Helper()
	code, out, errs := h.run(args...)
	if code != want {
		h.t.Fatalf("macfit %s: exit %d, want %d\nstdout: %s\nstderr: %s", strings.Join(args, " "), code, want, out, errs)
	}
	return out, errs
}

func (h *harness) write(rel, content string, mode os.FileMode) string {
	h.t.Helper()
	p := filepath.Join(h.home, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		h.t.Fatal(err)
	}
	if err := os.Chmod(p, mode); err != nil {
		h.t.Fatal(err)
	}
	return p
}

func (h *harness) read(rel string) string {
	h.t.Helper()
	b, err := os.ReadFile(filepath.Join(h.home, rel))
	if err != nil {
		h.t.Fatal(err)
	}
	return string(b)
}

func (h *harness) mode(rel string) os.FileMode {
	h.t.Helper()
	info, err := os.Stat(filepath.Join(h.home, rel))
	if err != nil {
		h.t.Fatal(err)
	}
	return info.Mode().Perm()
}

func (h *harness) openStore() *lockbox.Store {
	h.t.Helper()
	st, err := h.app.openStore(h.store)
	if err != nil {
		h.t.Fatal(err)
	}
	return st
}

func (h *harness) generation() uint64 {
	h.t.Helper()
	b, err := os.ReadFile(h.store)
	if err != nil {
		h.t.Fatal(err)
	}
	hdr, err := lockbox.ParseHeader(b)
	if err != nil {
		h.t.Fatal(err)
	}
	return hdr.Generation
}

func (h *harness) pointer() string {
	h.t.Helper()
	b, err := os.ReadFile(h.app.pointerFile())
	if err != nil {
		return ""
	}
	return string(b)
}

func TestVersionAndHelp(t *testing.T) {
	h := newHarness(t)
	for _, arg := range []string{"--version", "-v", "v", "version"} {
		if code, out, errs := h.runRaw(arg); code != 0 || out != "macfit v1.5.0\n" || errs != "" {
			t.Fatalf("%s: code %d stdout %q stderr %q", arg, code, out, errs)
		}
	}
	if code, out, _ := h.runRaw(); code != 1 || !strings.Contains(out, "store file: missing") {
		t.Fatalf("bare invocation must run st: code %d out %q", code, out)
	}
	if code, out, _ := h.run(); code != 1 || !strings.Contains(out, "store: "+h.store+" (flag)") {
		t.Fatalf("bare invocation with -s must run st on that store: code %d out %q", code, out)
	}
	if code, out, _ := h.runRaw("help"); code != 0 || !strings.Contains(out, "macfit key show") {
		t.Fatalf("help: code %d out %q", code, out)
	}
	if code, _, errs := h.run("bogus"); code != 2 || !strings.Contains(errs, "unknown command") {
		t.Fatalf("unknown command: code %d stderr %q", code, errs)
	}
	if code, _, errs := h.run("add", "--nope", "x"); code != 2 || !strings.Contains(errs, "usage") {
		t.Fatalf("unknown flag: code %d stderr %q", code, errs)
	}
}

func TestHelpLayoutMatchesTheOtherUtilities(t *testing.T) {
	h := newHarness(t)
	for _, arg := range []string{"help", "-h", "-?", "--help", "h"} {
		code, out, _ := h.runRaw(arg)
		if code != 0 {
			t.Fatalf("%s: code %d", arg, code)
		}
		lines := strings.Split(out, "\n")
		if lines[0] != "macfit v1.5.0" {
			t.Fatalf("%s: first line %q", arg, lines[0])
		}
		if lines[1] != "Keep Mac config files in one encrypted store and restore them on any Mac." {
			t.Fatalf("%s: second line %q", arg, lines[1])
		}
		last := -1
		for _, section := range []string{"\nOverview\n", "\nUsage\n", "\nOptions\n", "\nNotes\n"} {
			idx := strings.Index(out, section)
			if idx < 0 || idx < last {
				t.Fatalf("%s: section %q missing or out of order", arg, strings.TrimSpace(section))
			}
			last = idx
		}
		for _, want := range []string{"  -N, --new ", "  -h, -?, --help     Show this help message and exit", "Store path order: -s, then MACFIT_STORE", "plan the restore, or write it with -f", "Print the pull plan; the default", "  macfit render [-o DIR] [-a] [-f]", "  macfit cat TARGET [-H HOST]", "  -o, --out DIR ", "  -a, --all ", "  macfit [st] ", "  macfit set TARGET [flags]", "  -g, --global ", "  -m, --mode MODE ", "  -F, --from WHICH ", "register live files for this Mac"} {
			if !strings.Contains(out, want) {
				t.Fatalf("%s: help lacks %q", arg, want)
			}
		}
	}
}

func TestHelpHeaderAndHeadingsAreColoredLikeSkout(t *testing.T) {
	h := newHarness(t)
	plain := color.ClearCode(usage())
	defer color.SetEnabled(true)()
	_, out, _ := h.runRaw("help")
	lines := strings.Split(out, "\n")
	if lines[0] != color.Bold(color.Gra10("macfit"))+" v1.5.0" {
		t.Fatalf("first line %q", lines[0])
	}
	if lines[1] != color.Gra5("Keep Mac config files in one encrypted store and restore them on any Mac.") {
		t.Fatalf("second line %q", lines[1])
	}
	for _, name := range []string{"Overview", "Usage", "Options", "Notes"} {
		if !strings.Contains(out, "\n"+color.Bold(color.Gra10(name))+"\n") {
			t.Fatalf("heading %s is not bold white: %q", name, out)
		}
	}
	if color.ClearCode(out) != plain {
		t.Fatal("stripping escapes does not yield the plain help")
	}
	restore := color.SetEnabled(false)
	_, out, _ = h.runRaw("help")
	restore()
	if strings.Contains(out, "\x1b[") || out != plain {
		t.Fatalf("color disabled: %q", out)
	}
}

func TestReadmeUsageBlockEqualsHelp(t *testing.T) {
	b, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	readme := string(b)
	start := strings.Index(readme, "### Usage\n\n```text\n")
	if start < 0 {
		t.Fatal("README has no ### Usage block")
	}
	start += len("### Usage\n\n```text\n")
	end := strings.Index(readme[start:], "```")
	if end < 0 {
		t.Fatal("README usage block is not closed")
	}
	if got, want := readme[start:start+end], color.ClearCode(usage()); got != want {
		t.Fatalf("README usage block differs from help\n--- README\n%s\n--- help\n%s", got, want)
	}
}

func TestPlatformGuard(t *testing.T) {
	h := newHarness(t)
	h.app.goos = "linux"
	for _, verb := range []string{"init", "st", "add", "set", "rm", "ls", "push", "pull", "diff", "key", "render", "cat"} {
		code, _, errs := h.run(verb)
		if code != 1 || !strings.Contains(errs, "macfit supports macOS only") {
			t.Fatalf("%s on linux: code %d stderr %q", verb, code, errs)
		}
	}
	if code, _, _ := h.runRaw("--version"); code != 0 {
		t.Fatalf("--version on linux: code %d", code)
	}
	if code, _, _ := h.runRaw("help"); code != 0 {
		t.Fatalf("help on linux: code %d", code)
	}
}

func TestStoreResolutionOrder(t *testing.T) {
	h := newHarness(t)
	def := filepath.Join(h.home, ".local", "share", "macfit", "macfit.store")
	_, out, _ := h.runRaw("key", "show")
	if !strings.HasPrefix(out, "store: "+def+" (default)\n") {
		t.Fatalf("default: %q", out)
	}
	if err := os.MkdirAll(filepath.Dir(h.app.pointerFile()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h.app.pointerFile(), []byte(filepath.Join(h.root, "pointed.store")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, out, _ = h.runRaw("key", "show")
	if !strings.HasPrefix(out, "store: "+filepath.Join(h.root, "pointed.store")+" (pointer)\n") {
		t.Fatalf("pointer: %q", out)
	}
	h.app.storeEnv = filepath.Join(h.root, "env.store")
	_, out, _ = h.runRaw("key", "show")
	if !strings.HasPrefix(out, "store: "+filepath.Join(h.root, "env.store")+" (env)\n") {
		t.Fatalf("env over pointer: %q", out)
	}
	_, out, _ = h.run("key", "show")
	if !strings.HasPrefix(out, "store: "+h.store+" (flag)\n") {
		t.Fatalf("flag over env: %q", out)
	}
}

func TestInitNewCreatesAndRefuses(t *testing.T) {
	h := newHarness(t)
	code, out, errs := h.runRaw("init", "-N")
	if code != 0 || !strings.Contains(out, "store created at ") {
		t.Fatalf("default init -N: code %d out %q err %q", code, out, errs)
	}
	dir := filepath.Join(h.home, ".local", "share", "macfit")
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("default folder mode %o, want 700", info.Mode().Perm())
	}
	if _, err := os.Stat(filepath.Join(dir, "macfit.store")); err != nil {
		t.Fatal("default store not created")
	}
	if h.pointer() != "" {
		t.Fatal("init -N at the default path must not write a pointer")
	}

	h.store = filepath.Join(h.root, "missing", "x.store")
	if _, errs := h.mustFail(1, "init", "-N"); !strings.Contains(errs, "does not exist") {
		t.Fatalf("missing parent: %q", errs)
	}
	if _, err := os.Stat(filepath.Join(h.root, "missing")); !os.IsNotExist(err) {
		t.Fatal("init -N created the missing parent")
	}

	h.store = filepath.Join(h.root, "synced", "macfit.store")
	out = h.mustRun("init", "-N")
	if !strings.Contains(out, "store path remembered in "+h.app.pointerFile()) {
		t.Fatalf("init -N -s: %q", out)
	}
	pinfo, err := os.Stat(h.app.pointerFile())
	if err != nil {
		t.Fatal(err)
	}
	if pinfo.Mode().Perm() != 0o600 || h.pointer() != h.store+"\n" {
		t.Fatalf("pointer mode %o content %q", pinfo.Mode().Perm(), h.pointer())
	}
	before, _ := os.ReadFile(h.store)
	if _, errs := h.mustFail(1, "init", "-N"); !strings.Contains(errs, "already exists") {
		t.Fatalf("init -N over existing: %q", errs)
	}
	after, _ := os.ReadFile(h.store)
	if !bytes.Equal(before, after) {
		t.Fatal("init -N over an existing store changed it")
	}
}

func TestInitUnlocksAndRemembers(t *testing.T) {
	h := newHarness(t)
	if _, errs := h.mustFail(1, "init"); !strings.Contains(errs, "init -N") {
		t.Fatalf("init without store: %q", errs)
	}
	h.mustRun("init", "-N")
	var savedID string
	var savedKey []byte
	for id, k := range h.keys.Keys {
		savedID, savedKey = id, k
	}

	other := newHarness(t)
	other.store = h.store
	out := other.mustRun("init")
	if !strings.Contains(out, "saved to the login keychain") || !strings.Contains(out, "store path remembered") {
		t.Fatalf("other Mac init: %q", out)
	}
	if got := other.keys.Keys[savedID]; !bytes.Equal(got, savedKey) {
		t.Fatal("other Mac recovered a different key")
	}
	if code, _, errs := other.runRaw("ls"); code != 0 {
		t.Fatalf("ls through the pointer: code %d stderr %q", code, errs)
	}
	if out := other.mustRun("init"); !strings.Contains(out, "already unlocked") {
		t.Fatalf("init on an unlocked store: %q", out)
	}

	wrong := newHarness(t)
	wrong.store = h.store
	wrong.app.readSecret = func(string) ([]byte, error) { return []byte("nope"), nil }
	if _, errs := wrong.mustFail(1, "init"); !strings.Contains(errs, "wrong passphrase") {
		t.Fatalf("wrong passphrase: %q", errs)
	}
	if len(wrong.keys.Keys) != 0 || wrong.pointer() != "" {
		t.Fatal("wrong passphrase must save neither key nor pointer")
	}
	if _, errs := wrong.mustFail(1, "ls"); !strings.Contains(errs, "no key for") {
		t.Fatalf("ls without key: %q", errs)
	}
}

func TestRelativeStoreFlagIsMadeAbsolute(t *testing.T) {
	h := newHarness(t)
	if err := os.MkdirAll(filepath.Join(h.root, "rel"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(h.root)
	if code, _, errs := h.runRaw("-s", "rel/macfit.store", "init", "-N"); code != 0 {
		t.Fatalf("relative -s: code %d stderr %q", code, errs)
	}
	want, _ := filepath.EvalSymlinks(filepath.Join(h.root, "rel"))
	got, _ := filepath.EvalSymlinks(filepath.Dir(strings.TrimSpace(h.pointer())))
	if got != want || !filepath.IsAbs(strings.TrimSpace(h.pointer())) {
		t.Fatalf("pointer holds %q, want a path under %q", h.pointer(), want)
	}
}

func TestInitRefusalsChangeNothing(t *testing.T) {
	h := newHarness(t)
	h.app.isTerminal = func() bool { return false }
	if _, errs := h.mustFail(1, "init", "-N"); !strings.Contains(errs, "init needs a terminal") {
		t.Fatalf("non-terminal: %q", errs)
	}
	h.app.isTerminal = func() bool { return true }
	answers := [][]byte{[]byte("one"), []byte("two")}
	h.app.readSecret = func(string) ([]byte, error) {
		a := answers[0]
		answers = answers[1:]
		return a, nil
	}
	if _, errs := h.mustFail(1, "init", "-N"); !strings.Contains(errs, "do not match") {
		t.Fatalf("mismatch: %q", errs)
	}
	h.app.readSecret = func(string) ([]byte, error) { return []byte("  "), nil }
	if _, errs := h.mustFail(1, "init", "-N"); !strings.Contains(errs, "must not be empty") {
		t.Fatalf("empty: %q", errs)
	}
	if _, err := os.Stat(h.store); !os.IsNotExist(err) {
		t.Fatal("a refused init wrote a store")
	}
	if h.pointer() != "" {
		t.Fatal("a refused init wrote a pointer")
	}
}

func TestAddPushDiffPullRoundTrip(t *testing.T) {
	h := newHarness(t)
	h.mustRun("init", "-N")
	rel := ".config/git/config"
	live := h.write(rel, "[user]\n\tname = a\n", 0o600)
	const target = "$XDG_CONFIG_HOME/git/config"

	out := h.mustRun("add", live)
	if out != line("added", target+" (mode 0600, host a)") {
		t.Fatalf("add: %q", out)
	}
	out = h.mustRun("ls")
	if !strings.Contains(out, target) || !strings.Contains(out, "0600") {
		t.Fatalf("ls: %q", out)
	}
	if out := h.mustRun("diff"); out != "= "+target+"\n" {
		t.Fatalf("diff clean: %q", out)
	}
	if out := h.mustRun("push"); out != line("unchanged", target) {
		t.Fatalf("push unchanged: %q", out)
	}

	h.write(rel, "[user]\n\tname = b\n", 0o600)
	out, _ = h.mustFail(1, "diff")
	if out != "M "+target+"\n" {
		t.Fatalf("diff modified: %q", out)
	}
	out, _ = h.mustFail(1, "diff", "-V")
	if !strings.Contains(out, "-\tname = a") || !strings.Contains(out, "+\tname = b") || !strings.Contains(out, "@@ -1,2 +1,2 @@") {
		t.Fatalf("diff -V: %q", out)
	}
	if out := h.mustRun("push"); out != line("updated", target) {
		t.Fatalf("push updated: %q", out)
	}
	st := h.openStore()
	entries, _ := st.Entries()
	if n, _ := st.VersionCount(entries[0].ID); n != 2 {
		t.Fatalf("versions after second push: %d, want 2", n)
	}
	st.Close()
	if out := h.mustRun("push"); out != line("unchanged", target) {
		t.Fatalf("push after update: %q", out)
	}

	if err := os.Remove(live); err != nil {
		t.Fatal(err)
	}
	if out, _ := h.mustFail(1, "diff"); out != "? "+target+"\n" {
		t.Fatalf("diff missing: %q", out)
	}
	if out := h.mustRun("push"); out != line("missing", target) {
		t.Fatalf("push missing: %q", out)
	}
	if out := h.mustRun("pull"); out != line("would write", target) {
		t.Fatalf("pull plan: %q", out)
	}
	if _, err := os.Stat(live); !os.IsNotExist(err) {
		t.Fatal("pull without -f wrote the file")
	}
	if out := h.mustRun("pull", "-f"); out != line("restored", target) {
		t.Fatalf("pull -f: %q", out)
	}
	if got := h.read(rel); got != "[user]\n\tname = b\n" {
		t.Fatalf("restored content %q", got)
	}
	if m := h.mode(rel); m != 0o600 {
		t.Fatalf("restored mode %o", m)
	}

	if err := os.RemoveAll(filepath.Join(h.home, ".config", "git")); err != nil {
		t.Fatal(err)
	}
	h.mustRun("pull", "-f")
	dirInfo, err := os.Stat(filepath.Join(h.home, ".config", "git"))
	if err != nil {
		t.Fatal(err)
	}
	if dirInfo.Mode().Perm() != 0o700 {
		t.Fatalf("parent of a 0600 entry created with mode %o, want 700", dirInfo.Mode().Perm())
	}

	h.write(rel, "local edit\n", 0o600)
	if out := h.mustRun("pull"); out != line("would overwrite", target) {
		t.Fatalf("pull plan on a differing file: %q", out)
	}
	if out := h.mustRun("pull", "-n", "-f"); out != line("would overwrite", target) {
		t.Fatalf("pull -n -f: %q", out)
	}
	if got := h.read(rel); got != "local edit\n" {
		t.Fatal("a plan wrote the file")
	}
	if out := h.mustRun("pull", "-f"); out != line("restored", target) {
		t.Fatalf("pull force: %q", out)
	}
	if got := h.read(rel); got != "[user]\n\tname = b\n" {
		t.Fatalf("forced content %q", got)
	}

	os.Chmod(live, 0o644)
	if out, _ := h.mustFail(1, "diff"); !strings.Contains(out, "M "+target+" (mode 0600 in store, 0644 live)") {
		t.Fatalf("diff mode: %q", out)
	}
	h.mustRun("push")
	if out := h.mustRun("diff"); out != "= "+target+"\n" {
		t.Fatalf("diff after mode push: %q", out)
	}

	if out := h.mustRun("rm", target); out != "removed "+target+", host a\n" {
		t.Fatalf("rm: %q", out)
	}
	if out := h.mustRun("diff"); out != "" {
		t.Fatalf("diff after rm: %q", out)
	}
	if _, errs := h.mustFail(1, "rm", target); !strings.Contains(errs, "not registered") {
		t.Fatalf("rm again: %q", errs)
	}
}

func TestAddSeveralFilesInOneRun(t *testing.T) {
	h := newHarness(t)
	h.mustRun("init", "-N")
	a := h.write(".bash_logout", "a\n", 0o644)
	b := h.write(".bashrc", "b\n", 0o644)
	c := h.write(".profile", "c\n", 0o600)
	gen := h.generation()
	out := h.mustRun("add", a, b, c)
	for _, want := range []string{line("added", "~/.bash_logout (mode 0644, host a)"), line("added", "~/.bashrc (mode 0644, host a)"), line("added", "~/.profile (mode 0600, host a)")} {
		if !strings.Contains(out, want) {
			t.Fatalf("multi add lacks %q: %q", want, out)
		}
	}
	if h.generation() != gen+1 {
		t.Fatalf("generation advanced by %d, want 1", h.generation()-gen)
	}
	d := h.write(".vimrc", "d\n", 0o644)
	e := h.write(".gitignore", "e\n", 0o644)
	gen = h.generation()
	out, errs := h.mustFail(1, "add", d, filepath.Join(h.home, "missing"), e)
	if !strings.Contains(out, line("added", "~/.vimrc (mode 0644, host a)")) || !strings.Contains(out, line("added", "~/.gitignore (mode 0644, host a)")) || !strings.Contains(errs, "missing") {
		t.Fatalf("partial add: out %q err %q", out, errs)
	}
	if h.generation() != gen+1 {
		t.Fatal("partial add must still save once")
	}
	if out := h.mustRun("ls"); strings.Count(out, "\n") != 6 {
		t.Fatalf("ls after adds: %q", out)
	}
}

func TestAddRefusesDuplicatesAndNonFiles(t *testing.T) {
	h := newHarness(t)
	h.mustRun("init", "-N")
	live := h.write(".bashrc", "x\n", 0o644)
	h.mustRun("add", live)
	_, errs := h.mustFail(1, "add", live)
	if !strings.Contains(errs, "already registered") || !strings.Contains(errs, "macfit push") {
		t.Fatalf("duplicate add: %q", errs)
	}
	if _, errs := h.mustFail(1, "add", filepath.Join(h.home, ".config")); !strings.Contains(errs, "not a regular file") {
		t.Fatalf("directory add: %q", errs)
	}
	link := filepath.Join(h.home, ".link")
	if err := os.Symlink(live, link); err != nil {
		t.Fatal(err)
	}
	if _, errs := h.mustFail(1, "add", link); !strings.Contains(errs, "not a regular file") {
		t.Fatalf("symlink add: %q", errs)
	}
	if _, errs := h.mustFail(1, "add", filepath.Join(h.home, "missing")); errs == "" {
		t.Fatal("missing file add must fail")
	}
	if out := h.mustRun("add", live, "-H", "b"); out != line("added", "~/.bashrc (mode 0644, host b)") {
		t.Fatalf("host-bound add: %q", out)
	}
	cfg := h.write(".config/git/config", "c\n", 0o644)
	if out := h.mustRun("add", cfg, "-l"); out != line("added", "~/.config/git/config (mode 0644, host a)") {
		t.Fatalf("literal add: %q", out)
	}
	if out := h.mustRun("push", cfg); out != line("unchanged", "~/.config/git/config") {
		t.Fatalf("push by live path of a literal entry: %q", out)
	}
	if _, errs := h.mustFail(1, "push", "~/nothing"); !strings.Contains(errs, "not registered") {
		t.Fatalf("push unknown target: %q", errs)
	}
}

func TestPullRefusesSymlink(t *testing.T) {
	h := newHarness(t)
	h.mustRun("init", "-N")
	live := h.write(".bashrc", "real\n", 0o644)
	h.mustRun("add", live)
	other := h.write("elsewhere", "other\n", 0o644)
	if err := os.Remove(live); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, live); err != nil {
		t.Fatal(err)
	}
	out, _ := h.mustFail(1, "pull", "-f")
	if out != line("symlink", "~/.bashrc (refusing to write through a link)") {
		t.Fatalf("pull -f onto symlink: %q", out)
	}
	if out := h.mustRun("pull"); out != line("symlink", "~/.bashrc (refusing to write through a link)") {
		t.Fatalf("pull plan onto symlink: %q", out)
	}
	info, err := os.Lstat(live)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink was replaced")
	}
	if got := h.read("elsewhere"); got != "other\n" {
		t.Fatal("symlink target was written through")
	}
}

func TestHostBoundEntriesWinOnTheirHost(t *testing.T) {
	h := newHarness(t)
	h.mustRun("init", "-N")
	live := h.write(".bashrc", "shared\n", 0o644)
	h.mustRun("add", live, "-g")
	h.write(".bashrc", "for a\n", 0o644)
	h.mustRun("add", live, "-H", "a")
	h.write(".bashrc", "for b\n", 0o644)
	h.mustRun("add", live, "-H", "b")

	os.Remove(live)
	h.mustRun("pull", "-f")
	if got := h.read(".bashrc"); got != "for a\n" {
		t.Fatalf("host a pulled %q", got)
	}
	if out := h.mustRun("diff"); out != "= ~/.bashrc\n" {
		t.Fatalf("host a diff: %q", out)
	}

	h.app.host = "c"
	os.Remove(live)
	h.mustRun("pull", "-f")
	if got := h.read(".bashrc"); got != "shared\n" {
		t.Fatalf("host c pulled %q", got)
	}
	h.app.host = "a"
	if out, _ := h.mustFail(1, "diff"); out != "M ~/.bashrc\n" {
		t.Fatalf("host a diff against shared content: %q", out)
	}

	out := h.mustRun("ls")
	if strings.Count(out, "~/.bashrc") != 3 {
		t.Fatalf("ls must list every variant: %q", out)
	}
	if out := h.mustRun("rm", live); out != "removed ~/.bashrc, host a\n" {
		t.Fatalf("rm picks this host's entry: %q", out)
	}
	if out := h.mustRun("rm", live, "-H", "b"); out != "removed ~/.bashrc, host b\n" {
		t.Fatalf("rm -H: %q", out)
	}
	if out := h.mustRun("rm", live); out != "removed ~/.bashrc\n" {
		t.Fatalf("rm falls back to the unbound entry: %q", out)
	}
}

func TestConflictCopyWarning(t *testing.T) {
	h := newHarness(t)
	h.mustRun("init", "-N")
	for _, n := range []string{"macfit 2.store", "macfit (1).store"} {
		if err := os.WriteFile(filepath.Join(filepath.Dir(h.store), n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	_, _, errs := h.run("ls")
	if !strings.Contains(errs, "sync conflict copies") || !strings.Contains(errs, "macfit 2.store") || !strings.Contains(errs, "macfit (1).store") {
		t.Fatalf("warning: %q", errs)
	}
}

// outcomeFixture builds a store with one identical, one differing, one
// missing, and one symlinked live file, plus one file with a mode change.
func outcomeFixture(h *harness) {
	h.mustRun("init", "-N")
	same := h.write(".profile", "same\n", 0o644)
	changed := h.write(".bashrc", "one\n", 0o644)
	gone := h.write(".vimrc", "gone\n", 0o644)
	linked := h.write(".zshrc", "linked\n", 0o644)
	h.mustRun("add", same, changed, gone, linked)
	h.write(".bashrc", "two\n", 0o644)
	os.Remove(gone)
	other := h.write("elsewhere", "other\n", 0o644)
	os.Remove(linked)
	if err := os.Symlink(other, linked); err != nil {
		h.t.Fatal(err)
	}
}

func TestStatusLinesAreColoredByOutcome(t *testing.T) {
	h := newHarness(t)
	outcomeFixture(h)
	defer color.SetEnabled(true)()
	wantColor := map[string]func(any) string{
		"=": color.Gra5, "unchanged": color.Gra5,
		"M": color.Yel5, "would overwrite": color.Yel5, "missing": color.Yel5,
		"?":           color.Org5,
		"would write": color.Grn5, "restored": color.Grn5, "added": color.Grn5, "updated": color.Grn5,
		"symlink": color.Red5,
	}
	seen := map[string]bool{}
	check := func(out string) {
		for l := range strings.SplitSeq(strings.TrimSuffix(out, "\n"), "\n") {
			plain := color.ClearCode(l)
			if plain == "" {
				continue
			}
			word := strings.TrimSpace(plain[:min(len(plain), statusWidth)])
			if plain[0] == '=' || plain[0] == 'M' || plain[0] == '?' {
				word = plain[:1]
			}
			paintFn, known := wantColor[word]
			if !known {
				continue
			}
			if l != paintFn(plain) {
				t.Fatalf("line %q is not colored as %q should be", l, word)
			}
			seen[word] = true
		}
	}
	_, out, _ := h.run("diff")
	check(out)
	_, out, _ = h.run("pull")
	check(out)
	_, out, _ = h.run("push")
	check(out)
	_, out, _ = h.run("pull", "-f")
	check(out)
	extra := h.write(".extra", "x\n", 0o644)
	_, out, _ = h.run("add", extra)
	check(out)
	for word := range wantColor {
		if !seen[word] {
			t.Fatalf("outcome %q never appeared in the fixture output", word)
		}
	}

	restore := color.SetEnabled(false)
	defer restore()
	for _, args := range [][]string{{"diff"}, {"pull"}, {"push"}} {
		if _, out, _ := h.run(args...); strings.Contains(out, "\x1b[") {
			t.Fatalf("%v with color disabled: %q", args, out)
		}
	}
}

func TestStatusWordsArePaddedToOneColumn(t *testing.T) {
	h := newHarness(t)
	outcomeFixture(h)
	for _, args := range [][]string{{"pull"}, {"push"}} {
		_, out, _ := h.run(args...)
		for l := range strings.SplitSeq(strings.TrimSuffix(out, "\n"), "\n") {
			if len(l) <= statusWidth || l[statusWidth-1] != ' ' || l[statusWidth-2] != ' ' || l[statusWidth] != '~' {
				t.Fatalf("%v: target does not start at column %d: %q", args, statusWidth, l)
			}
		}
	}
	if statusWidth != len("would overwrite")+2 {
		t.Fatalf("statusWidth %d", statusWidth)
	}
	_, out, _ := h.run("diff")
	for l := range strings.SplitSeq(strings.TrimSuffix(out, "\n"), "\n") {
		if len(l) < 3 || l[1] != ' ' || l[2] != '~' {
			t.Fatalf("diff marker line %q", l)
		}
	}
}

func TestDiffBlockIsQuietAndSetOff(t *testing.T) {
	h := newHarness(t)
	h.mustRun("init", "-N")
	same := h.write(".profile", "same\n", 0o644)
	changed := h.write(".bashrc", "a\nb\nc\n", 0o644)
	h.mustRun("add", same, changed)
	h.write(".bashrc", "a\nx\nc\n", 0o644)

	restore := color.SetEnabled(false)
	_, plain, _ := h.run("diff", "-V")
	restore()
	want := "M ~/.bashrc\n" +
		"\n" +
		"--- ~/.bashrc (store)\n" +
		"+++ " + filepath.Join(h.home, ".bashrc") + " (live)\n" +
		"@@ -1,3 +1,3 @@\n" +
		" a\n" +
		"-b\n" +
		"+x\n" +
		" c\n" +
		"\n" +
		"= ~/.profile\n"
	if plain != want {
		t.Fatalf("plain diff -V\n got %q\nwant %q", plain, want)
	}

	defer color.SetEnabled(true)()
	_, out, _ := h.run("diff", "-V")
	if color.ClearCode(out) != plain {
		t.Fatal("colored diff -V does not strip to the plain output")
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	expect := map[int]func(any) string{2: color.Gra4, 3: color.Gra4, 4: color.Gra4, 5: color.Gra5, 6: color.Yel8, 7: color.Yel8, 8: color.Gra5, 10: color.Gra5}
	for i, paintFn := range expect {
		if lines[i] != paintFn(color.ClearCode(lines[i])) {
			t.Fatalf("diff line %d %q has the wrong color", i, lines[i])
		}
	}
}

func TestPullPlansByDefaultAndWritesWithForce(t *testing.T) {
	h := newHarness(t)
	outcomeFixture(h)
	want := line("unchanged", "~/.profile") + line("would overwrite", "~/.bashrc") +
		line("would write", "~/.vimrc") + line("symlink", "~/.zshrc (refusing to write through a link)")
	sortLines := func(s string) string {
		ls := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
		sort.Strings(ls)
		return strings.Join(ls, "\n") + "\n"
	}
	for _, args := range [][]string{{"pull"}, {"pull", "-n"}, {"pull", "-n", "-f"}} {
		code, out, _ := h.run(args...)
		if code != 0 || sortLines(out) != sortLines(want) {
			t.Fatalf("%v: code %d out %q", args, code, out)
		}
	}
	if h.read(".bashrc") != "two\n" {
		t.Fatal("a plan overwrote .bashrc")
	}
	if _, err := os.Stat(filepath.Join(h.home, ".vimrc")); !os.IsNotExist(err) {
		t.Fatal("a plan wrote .vimrc")
	}

	code, out, _ := h.run("pull", "-f")
	wantWrite := line("unchanged", "~/.profile") + line("restored", "~/.bashrc") +
		line("restored", "~/.vimrc") + line("symlink", "~/.zshrc (refusing to write through a link)")
	if code != 1 || sortLines(out) != sortLines(wantWrite) {
		t.Fatalf("pull -f: code %d out %q", code, out)
	}
	if h.read(".bashrc") != "one\n" || h.read(".vimrc") != "gone\n" {
		t.Fatal("pull -f did not restore the files")
	}
	if h.mode(".vimrc") != 0o644 {
		t.Fatalf("restored mode %o", h.mode(".vimrc"))
	}
	h.mustRun("rm", "~/.zshrc")
	if code, _, _ := h.run("pull", "-f"); code != 0 {
		t.Fatalf("pull -f without a refusal: code %d", code)
	}
}

func TestUnifiedDiff(t *testing.T) {
	got := unifiedDiff("s", "l", []byte("a\nb\nc\n"), []byte("a\nx\nc\nd\n"))
	want := "--- s\n+++ l\n@@ -1,3 +1,4 @@\n a\n-b\n+x\n c\n+d\n"
	if got != want {
		t.Fatalf("diff\n got %q\nwant %q", got, want)
	}
	if got := unifiedDiff("s", "l", nil, []byte("new\n")); !strings.Contains(got, "+new\n") {
		t.Fatalf("empty store side: %q", got)
	}
}

func TestParseArgs(t *testing.T) {
	specs := []flagSpec{{"-H", "--host", true}, {"-l", "--literal", false}}
	flags, pos, err := parseArgs([]string{"a", "-H", "np10", "--literal", "b", "--", "-c"}, specs)
	if err != nil || flags["--host"] != "np10" || flags["--literal"] != "true" || strings.Join(pos, ",") != "a,b,-c" {
		t.Fatalf("parse: %v %v %v", flags, pos, err)
	}
	if _, _, err := parseArgs([]string{"--host"}, specs); err == nil {
		t.Fatal("missing value accepted")
	}
	if _, _, err := parseArgs([]string{"--literal=x"}, specs); err == nil {
		t.Fatal("value on a boolean flag accepted")
	}
	flags, _, _ = parseArgs([]string{"--host=x"}, specs)
	if flags["--host"] != "x" {
		t.Fatalf("inline value: %v", flags)
	}
}

func TestStoreEnvironmentAndFlagErrors(t *testing.T) {
	h := newHarness(t)
	h.app.storeEnv = filepath.Join(h.root, "synced", "env.store")
	if code, out, errs := h.runRaw("init", "-N"); code != 0 || !strings.Contains(out, "env.store") || strings.Contains(out, "remembered") {
		t.Fatalf("MACFIT_STORE: code %d out %q err %q", code, out, errs)
	}
	h.app.storeEnv = ""
	if code, _, errs := h.runRaw("ls"); code != 1 || !strings.Contains(errs, filepath.Join(".local", "share", "macfit", "macfit.store")) {
		t.Fatalf("default store path: code %d err %q", code, errs)
	}
	if code, _, _ := h.runRaw("ls", "--store"); code != 2 {
		t.Fatalf("--store without value: code %d", code)
	}
	if code, _, _ := h.run("init", "-N", "extra"); code != 2 {
		t.Fatalf("init with positional: code %d", code)
	}
}

func TestEmptyFileRoundTrip(t *testing.T) {
	h := newHarness(t)
	h.mustRun("init", "-N")
	live := h.write(".hushlogin", "", 0o644)
	if out := h.mustRun("add", live); out != line("added", "~/.hushlogin (mode 0644, host a)") {
		t.Fatalf("add empty: %q", out)
	}
	if out := h.mustRun("diff"); out != "= ~/.hushlogin\n" {
		t.Fatalf("diff empty: %q", out)
	}
	if err := os.Remove(live); err != nil {
		t.Fatal(err)
	}
	if out := h.mustRun("pull", "-f"); out != line("restored", "~/.hushlogin") {
		t.Fatalf("pull empty: %q", out)
	}
	if got := h.read(".hushlogin"); got != "" {
		t.Fatalf("restored empty file holds %q", got)
	}
	if out := h.mustRun("push"); out != line("unchanged", "~/.hushlogin") {
		t.Fatalf("push empty: %q", out)
	}
}

func TestStatusScreen(t *testing.T) {
	h := newHarness(t)
	h.mustRun("init", "-N")
	same := h.write(".profile", "same\n", 0o644)
	changed := h.write(".bashrc", "one\n", 0o644)
	other := h.write(".vimrc", "elsewhere\n", 0o644)
	h.mustRun("add", same, changed)
	h.mustRun("add", other, "-H", "other")
	h.write(".bashrc", "two\n", 0o644)
	info, err := os.Stat(h.store)
	if err != nil {
		t.Fatal(err)
	}
	out, _ := h.mustFail(1, "st")
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	want := []string{
		"store: " + h.store + " (flag)",
		"remembered: " + h.app.pointerFile() + " -> " + h.store,
		fmt.Sprintf("store file: present, %d bytes, generation %d, modified %s", info.Size(), h.generation(), info.ModTime().Local().Format(time.DateTime)),
		"key id: ",
		"login keychain: present",
		"store opens: yes",
		"host: a",
		"entries: 3 total, 2 for this Mac",
		"conflict copies: none",
		"drift: M 1, ? 0",
	}
	if len(lines) != len(want) {
		t.Fatalf("st printed %d lines, want %d:\n%s", len(lines), len(want), out)
	}
	for i, w := range want {
		if i == 3 {
			if !strings.HasPrefix(lines[i], w) || len(lines[i]) != len(w)+32 {
				t.Fatalf("line %d %q, want a 32-hex key id", i, lines[i])
			}
			continue
		}
		if lines[i] != w {
			t.Fatalf("line %d %q, want %q", i, lines[i], w)
		}
	}

	plain := out
	defer color.SetEnabled(true)()
	_, out, _ = h.run("st")
	if color.ClearCode(out) != plain {
		t.Fatal("colored st does not strip to the plain screen")
	}
	colored := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if colored[0] != "store: "+color.Gra4(h.store+" (flag)") {
		t.Fatalf("store value not grey: %q", colored[0])
	}
	if colored[5] != "store opens: "+color.Grn5("yes") {
		t.Fatalf("store opens not green: %q", colored[5])
	}
	if colored[9] != "drift: "+color.Red5("M 1, ? 0") {
		t.Fatalf("colored drift line: %q", colored[9])
	}
	restore := color.SetEnabled(false)
	defer restore()

	copyPath := filepath.Join(filepath.Dir(h.store), "macfit 2.store")
	if err := os.WriteFile(copyPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, _ := h.mustFail(1, "st"); !strings.Contains(out, "conflict copies: "+copyPath+"\n") {
		t.Fatalf("conflict copy line: %q", out)
	}
	color.SetEnabled(true)
	if _, out, _ := h.run("st"); !strings.Contains(out, "conflict copies: "+color.Yel5(copyPath)+"\n") {
		t.Fatalf("conflict copy not yellow: %q", out)
	}
	color.SetEnabled(false)
	os.Remove(copyPath)

	saved := h.keys.Keys
	h.keys.Keys = map[string][]byte{}
	out, _ = h.mustFail(1, "st")
	for _, w := range []string{"login keychain: missing", "store opens: no (key missing)", "host: a", "entries: unknown", "conflict copies: none", "drift: unknown"} {
		if !strings.Contains(out, w+"\n") {
			t.Fatalf("st without key lacks %q: %q", w, out)
		}
	}
	color.SetEnabled(true)
	if _, out, _ := h.run("st"); !strings.Contains(out, "store opens: "+color.Red5("no (key missing)")+"\n") {
		t.Fatalf("store opens not red: %q", out)
	}
	color.SetEnabled(false)
	h.keys.Keys = saved

	fresh := newHarness(t)
	out, _ = fresh.mustFail(1, "st")
	for _, w := range []string{"remembered: none", "store file: missing", "key id: unknown", "login keychain: unknown", "store opens: no (store file missing)", "entries: unknown", "drift: unknown"} {
		if !strings.Contains(out, w+"\n") {
			t.Fatalf("st without store lacks %q: %q", w, out)
		}
	}
	if code, _, _ := h.run("st", "extra"); code != 2 {
		t.Fatalf("st with an argument: code %d", code)
	}
}

func TestStatusDriftAgreesWithDiff(t *testing.T) {
	h := newHarness(t)
	outcomeFixture(h)
	_, diffOut, _ := h.run("diff")
	counts := map[byte]int{}
	for l := range strings.SplitSeq(strings.TrimSuffix(diffOut, "\n"), "\n") {
		counts[l[0]]++
	}
	want := fmt.Sprintf("drift: M %d, ? %d", counts['M'], counts['?'])
	if counts['M'] < 2 || counts['?'] == 0 {
		t.Fatalf("fixture must yield a symlink counted as M and a missing file: %q", diffOut)
	}
	if out, _ := h.mustFail(1, "st"); !strings.Contains(out, want+"\n") {
		t.Fatalf("st drift %q not found in %q", want, out)
	}
	defer color.SetEnabled(true)()
	if _, out, _ := h.run("st"); !strings.Contains(out, "drift: "+color.Red5(fmt.Sprintf("M %d, ? %d", counts['M'], counts['?']))+"\n") {
		t.Fatalf("drift verdict not red: %q", out)
	}
	os.Remove(filepath.Join(h.home, ".zshrc"))
	h.mustRun("rm", "~/.zshrc")
	h.mustRun("pull", "-f")
	restore := color.SetEnabled(false)
	defer restore()
	if out := h.mustRun("st"); !strings.Contains(out, "drift: none\n") {
		t.Fatalf("clean st: %q", out)
	}
	color.SetEnabled(true)
	if _, out, _ := h.run("st"); !strings.Contains(out, "drift: "+color.Grn5("none")+"\n") {
		t.Fatalf("clean st not green: %q", out)
	}
}

func TestPushAndDiffRefuseSymlinks(t *testing.T) {
	h := newHarness(t)
	h.mustRun("init", "-N")
	live := h.write(".bashrc", "real\n", 0o644)
	h.mustRun("add", live)
	other := h.write("elsewhere", "other\n", 0o644)
	os.Remove(live)
	if err := os.Symlink(other, live); err != nil {
		t.Fatal(err)
	}
	out, _ := h.mustFail(1, "push")
	if out != line("symlink", "~/.bashrc (refusing to read through a link)") {
		t.Fatalf("push through symlink: %q", out)
	}
	st := h.openStore()
	entries, _ := st.Entries()
	if n, _ := st.VersionCount(entries[0].ID); n != 1 {
		t.Fatalf("push through a symlink stored a version: %d", n)
	}
	st.Close()
	if out, _ := h.mustFail(1, "diff"); out != "M ~/.bashrc (live is a symlink)\n" {
		t.Fatalf("diff on symlink: %q", out)
	}
	if out, _ := h.mustFail(1, "diff", "-V"); out != "M ~/.bashrc (live is a symlink)\n" {
		t.Fatalf("diff -V on symlink printed a block: %q", out)
	}
	defer color.SetEnabled(true)()
	if _, out, _ := h.run("push"); out != color.Red5(line("symlink", "~/.bashrc (refusing to read through a link)")[:len(line("symlink", "~/.bashrc (refusing to read through a link)"))-1])+"\n" {
		t.Fatalf("push symlink color: %q", out)
	}
	if _, out, _ := h.run("diff"); out != color.Yel5("M ~/.bashrc (live is a symlink)")+"\n" {
		t.Fatalf("diff symlink color: %q", out)
	}
}

// errReader makes readLine fail, proving a prompt was not reached.
func mustNotPrompt(t *testing.T) func(string) (string, error) {
	return func(string) (string, error) {
		t.Fatal("prompt reached unexpectedly")
		return "", errors.New("unreachable")
	}
}
