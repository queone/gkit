package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/queone/gkit/internal/lockbox"
)

// testKDF keeps passphrase derivation fast in tests.
var testKDF = lockbox.KDF{Time: 1, Memory: 8 * 1024, Threads: 1}

// line renders the plain, padded status line the CLI prints for an entry.
func line(word, name string) string {
	return fmt.Sprintf("%-*s%s\n", statusWidth, word, name)
}

// harness drives every command in-process against a private home, a
// private copy of the testdata specs, and a memory key store.
type harness struct {
	t     *testing.T
	app   *app
	out   *bytes.Buffer
	errb  *bytes.Buffer
	root  string
	home  string
	specs string
	cfg   string
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

// copySpecs copies testdata/specs into dst and writes a configuration file
// beside it that adds content_version: test.
func copySpecs(t *testing.T, root string) (specs, cfg string) {
	t.Helper()
	specs = filepath.Join(root, "specs")
	if err := os.MkdirAll(specs, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir("testdata/specs")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join("testdata/specs", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(specs, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile("testdata/attune.yaml")
	if err != nil {
		t.Fatal(err)
	}
	cfg = filepath.Join(root, "attune.yaml")
	if err := os.WriteFile(cfg, append(raw, []byte("content_version: test\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	return specs, cfg
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	synced := filepath.Join(root, "synced")
	for _, d := range []string{filepath.Join(home, ".config"), synced} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	specs, cfg := copySpecs(t, root)
	h := &harness{t: t, out: &bytes.Buffer{}, errb: &bytes.Buffer{}, root: root, home: home, specs: specs, cfg: cfg,
		store: filepath.Join(synced, "attune.store"), keys: &lockbox.MemoryKeyStore{}}
	h.app = &app{
		stdout: h.out, stderr: h.errb, stdin: strings.NewReader(""), keys: h.keys, env: testEnv(home), host: "np10",
		kdf: testKDF, goos: "darwin",
		isTerminal: func() bool { return true },
		readSecret: func(string) ([]byte, error) { return []byte("pw"), nil },
		readLine:   func(string) (string, error) { return "", nil },
		editor:     func(string) error { return nil },
	}
	return h
}

// run executes attune with -t naming the harness store, appended so it
// follows the verb.
func (h *harness) run(args ...string) (int, string, string) {
	return h.runRaw(append(append([]string{}, args...), "-t", h.store)...)
}

// runRaw executes attune with the arguments as given, no -t added.
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
		h.t.Fatalf("attune %s: exit %d\nstdout: %s\nstderr: %s", strings.Join(args, " "), code, out, errs)
	}
	return out
}

func (h *harness) mustFail(want int, args ...string) (string, string) {
	h.t.Helper()
	code, out, errs := h.run(args...)
	if code != want {
		h.t.Fatalf("attune %s: exit %d, want %d\nstdout: %s\nstderr: %s", strings.Join(args, " "), code, want, out, errs)
	}
	return out, errs
}

// initAndAdd creates the store and captures the configuration and specs.
func (h *harness) initAndAdd() {
	h.t.Helper()
	h.mustRun("init", "-N")
	h.mustRun("add", "attune.yaml", h.cfg)
	h.mustRun("add", h.specs)
}

func (h *harness) spec(name string) string { return filepath.Join(h.specs, name) }

func (h *harness) read(p string) string {
	h.t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		h.t.Fatal(err)
	}
	return string(b)
}

// lsNames returns the NAME column of an ls listing.
func lsNames(out string) []string {
	var names []string
	for i, l := range strings.Split(strings.TrimSpace(out), "\n") {
		if i == 0 {
			continue
		}
		fields := strings.Fields(l)
		names = append(names, fields[len(fields)-1])
	}
	return names
}

const validateStoreLine = "attune validate: OK (6 specs) content=test\n"

var storeNames = []string{"app.yaml", "attune.yaml", "dns.yaml", "group.yaml", "resource-group.yaml", "role-assignment.yaml", "role-definition.yaml"}

func TestStoreRoundTripsTheSpecs(t *testing.T) {
	h := newHarness(t)
	out := h.mustRun("init", "-N")
	for _, want := range []string{"store created at " + h.store, "key saved to the login keychain", "store path remembered in " + h.app.pointerFile()} {
		if !strings.Contains(out, want) {
			t.Fatalf("init lacks %q: %q", want, out)
		}
	}
	if out := h.mustRun("add", "attune.yaml", h.cfg); out != line("added", "attune.yaml") {
		t.Fatalf("add attune.yaml %q", out)
	}
	out = h.mustRun("add", h.specs)
	if !strings.Contains(out, line("added", "app.yaml")) || !strings.Contains(out, line("added", "dns.yaml")) {
		t.Fatalf("add specs %q", out)
	}
	if got := lsNames(h.mustRun("ls")); strings.Join(got, ",") != strings.Join(storeNames, ",") {
		t.Fatalf("ls names %v, want %v", got, storeNames)
	}
	if got, want := h.mustRun("cat", "app.yaml"), h.read(h.spec("app.yaml")); got != want {
		t.Fatal("cat app.yaml differs from the file it came from")
	}
	if _, errs := h.mustFail(1, "add", "app.yaml", h.spec("app.yaml")); !strings.Contains(errs, "already registered") {
		t.Fatalf("second add: %q", errs)
	}
	if out := h.mustRun("rename", "app.yaml", "apps/app.yaml"); !strings.Contains(out, line("renamed", "app.yaml -> apps/app.yaml")) {
		t.Fatalf("rename output %q", out)
	}
	if got := h.mustRun("cat", "apps/app.yaml"); got != h.read(h.spec("app.yaml")) {
		t.Fatal("rename lost the stored content")
	}
	if _, errs := h.mustFail(1, "rename", "dns.yaml", "apps/app.yaml"); !strings.Contains(errs, "already registered") {
		t.Fatalf("rename onto a held name: %q", errs)
	}
	if _, errs := h.mustFail(2, "rename", "dns.yaml", "../x.yaml"); !strings.Contains(errs, "not a spec name") {
		t.Fatalf("rename to a bad name: %q", errs)
	}
	if out := h.mustRun("rm", "apps/app.yaml"); out != "removed apps/app.yaml\n" {
		t.Fatalf("rm output %q", out)
	}
	if got := lsNames(h.mustRun("ls")); len(got) != 6 || got[0] != "attune.yaml" {
		t.Fatalf("ls after rm %v", got)
	}
	if got := lsNames(h.mustRun("ls", "-b", "captured")); len(got) != 6 {
		t.Fatalf("ls -b captured %v", got)
	}
	if _, errs := h.mustFail(2, "ls", "-b", "size"); !strings.Contains(errs, "usage") {
		t.Fatalf("ls bad field: %q", errs)
	}
}

func TestValidateReadsOnlyTheStore(t *testing.T) {
	h := newHarness(t)
	h.initAndAdd()
	t.Chdir(t.TempDir())
	if code, out, errs := h.runRaw("validate"); code != 0 || out != validateStoreLine {
		t.Fatalf("remembered store: code %d out %q err %q", code, out, errs)
	}
	if out := h.mustRun("validate"); out != validateStoreLine {
		t.Fatalf("-t: %q", out)
	}
	if code, _, errs := h.runRaw("validate", "-s", "x"); code != 2 || !strings.Contains(errs, "unknown flag") {
		t.Fatalf("-s must be gone: code %d err %q", code, errs)
	}
	missing := filepath.Join(h.root, "missing.store")
	h.app.storeEnv = missing
	code, _, errs := h.runRaw("validate")
	if code != 1 || !strings.Contains(errs, missing) || !strings.Contains(errs, "attune init") {
		t.Fatalf("missing store: code %d err %q", code, errs)
	}
	h.app.storeEnv = ""
	if err := os.Remove(h.spec("app.yaml")); err != nil {
		t.Fatal(err)
	}
	if out := h.mustRun("validate"); out != validateStoreLine {
		t.Fatalf("validate after deleting a live file must not change: %q", out)
	}
}

func TestAddValidatesBeforeSaving(t *testing.T) {
	h := newHarness(t)
	h.initAndAdd()
	broken := filepath.Join(h.root, "broken.yaml")
	if err := os.WriteFile(broken, []byte("kind: [unterminated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, errs := h.mustFail(1, "add", "zones/broken.yaml", broken); !strings.Contains(errs, "parse zones/broken.yaml") {
		t.Fatalf("broken yaml: %q", errs)
	}
	if _, errs := h.mustFail(1, "add", "copies/dns.yaml", h.spec("dns.yaml")); !strings.Contains(errs, "duplicate resource key") {
		t.Fatalf("duplicate key: %q", errs)
	}
	if got := lsNames(h.mustRun("ls")); len(got) != 7 {
		t.Fatalf("refused adds changed the store: %v", got)
	}
	if _, errs := h.mustFail(2, "add", h.spec("dns.yaml")); !strings.Contains(errs, "attune add NAME FILE") {
		t.Fatalf("single file without a name: %q", errs)
	}
	h.mustRun("rm", "dns.yaml")
	h.app.stdin = strings.NewReader(h.read(h.spec("dns.yaml")))
	h.app.isTerminal = func() bool { return false }
	if out := h.mustRun("add", "zones/dns.yaml"); out != line("added", "zones/dns.yaml") {
		t.Fatalf("add from stdin %q", out)
	}
	if got := h.mustRun("cat", "zones/dns.yaml"); got != h.read(h.spec("dns.yaml")) {
		t.Fatal("stdin content differs")
	}
	h.app.isTerminal = func() bool { return true }
	if _, errs := h.mustFail(2, "add", "zones/other.yaml"); !strings.Contains(errs, "pipe the content") {
		t.Fatalf("stdin on a terminal: %q", errs)
	}
	if _, errs := h.mustFail(2, "add", "nope.txt"); !strings.Contains(errs, "not a spec name") {
		t.Fatalf("bad name: %q", errs)
	}
}

func TestStShowsStoreState(t *testing.T) {
	h := newHarness(t)
	if out, _ := h.mustFail(1, "st"); !strings.Contains(out, "store file: missing") || !strings.Contains(out, "store opens: no") {
		t.Fatalf("st before init %q", out)
	}
	h.initAndAdd()
	out := h.mustRun("st")
	for _, want := range []string{"store: " + h.store + " (flag)", "remembered: " + h.app.pointerFile() + " -> " + h.store, "login keychain: present", "store opens: yes", "entries: 7", "conflict copies: none", "drift: run attune plan to compare with Azure"} {
		if !strings.Contains(out, want) {
			t.Fatalf("st lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "bundle") || strings.Contains(out, "specs ") {
		t.Fatalf("st mentions a live directory: %s", out)
	}
	if _, errs := h.mustFail(2, "st", "extra"); !strings.Contains(errs, "takes no arguments") {
		t.Fatalf("st with an argument: %q", errs)
	}
}

func TestEveryCommandIsMacOSOnly(t *testing.T) {
	h := newHarness(t)
	h.initAndAdd()
	h.app.goos = "linux"
	for _, verb := range append([]string{"validate", "plan", "apply"}, storeVerbs...) {
		if code, _, errs := h.run(verb); code != 1 || !strings.Contains(errs, "attune store support is macOS only") {
			t.Fatalf("%s on linux: code %d stderr %q", verb, code, errs)
		}
	}
	if code, out, _ := h.runRaw("version"); code != 0 || out != "attune v"+programVersion+"\n" {
		t.Fatalf("version on linux: code %d out %q", code, out)
	}
	if code, out, _ := h.runRaw("help"); code != 0 || !strings.HasPrefix(out, "attune v") {
		t.Fatalf("help on linux: code %d out %q", code, out)
	}
}

func TestUnlockOnAnotherMac(t *testing.T) {
	h := newHarness(t)
	h.initAndAdd()
	h.app.keys = &lockbox.MemoryKeyStore{}
	h.app.env = testEnv(filepath.Join(h.root, "home2"))
	if _, errs := h.mustFail(1, "ls"); !strings.Contains(errs, "run `attune init`") {
		t.Fatalf("ls without the key: %q", errs)
	}
	if out := h.mustRun("init"); !strings.Contains(out, "saved to the login keychain") || !strings.Contains(out, "remembered in") {
		t.Fatalf("init unlock %q", out)
	}
	if got := lsNames(h.mustRun("ls")); len(got) != 7 {
		t.Fatalf("ls after unlock %v", got)
	}
	h.app.readSecret = func(string) ([]byte, error) { return []byte("wrong"), nil }
	h.app.keys = &lockbox.MemoryKeyStore{}
	if _, errs := h.mustFail(1, "init"); errs == "" {
		t.Fatal("init with the wrong passphrase succeeded")
	}
}
