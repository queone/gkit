package main

import (
	"os"
	"strings"
	"testing"

	"github.com/queone/gkit/internal/lockbox"
)

func TestKeyLifecycle(t *testing.T) {
	h := newHarness(t)
	h.mustRun("init", "-N")
	out := h.mustRun("key", "show")
	for _, want := range []string{"store: " + h.store + " (flag)", "key id: ", "login keychain: present", "store opens: yes"} {
		if !strings.Contains(out, want) {
			t.Fatalf("key show lacks %q: %q", want, out)
		}
	}
	if strings.Contains(out, "secret") {
		t.Fatalf("key show must not print secrets: %q", out)
	}

	h.app.readLine = func(string) (string, error) { return "n", nil }
	if out, _ := h.mustFail(1, "key", "rm"); !strings.Contains(out, "nothing deleted") {
		t.Fatalf("key rm answered n: %q", out)
	}
	if len(h.keys.Keys) != 1 {
		t.Fatal("key rm answered n deleted the key")
	}
	h.app.isTerminal = func() bool { return false }
	if _, errs := h.mustFail(1, "key", "rm"); !strings.Contains(errs, "needs a terminal") {
		t.Fatalf("key rm without terminal: %q", errs)
	}
	h.app.isTerminal = func() bool { return true }

	h.app.readLine = func(string) (string, error) { return "Y", nil }
	if out := h.mustRun("key", "rm"); !strings.Contains(out, "keychain item deleted") {
		t.Fatalf("key rm answered y: %q", out)
	}
	if len(h.keys.Keys) != 0 {
		t.Fatal("key rm answered y left the key")
	}
	if _, errs := h.mustFail(1, "ls"); !strings.Contains(errs, "run `macfit init`") {
		t.Fatalf("ls after key rm: %q", errs)
	}
	if out, _ := h.mustFail(1, "key", "show"); !strings.Contains(out, "login keychain: missing") {
		t.Fatalf("key show after rm: %q", out)
	}
	if _, errs := h.mustFail(1, "key", "rm", "-f"); !strings.Contains(errs, "no keychain item") {
		t.Fatalf("key rm with nothing to delete: %q", errs)
	}

	if out := h.mustRun("key", "restore"); !strings.Contains(out, "saved to the login keychain") {
		t.Fatalf("key restore: %q", out)
	}
	h.mustRun("ls")
	if _, errs := h.mustFail(1, "key", "restore"); !strings.Contains(errs, "already in the login keychain") {
		t.Fatalf("key restore twice: %q", errs)
	}

	h.app.readLine = mustNotPrompt(t)
	if out := h.mustRun("key", "rm", "-f"); !strings.Contains(out, "keychain item deleted") {
		t.Fatalf("key rm -f: %q", out)
	}
	if len(h.keys.Keys) != 0 {
		t.Fatal("key rm -f left the key")
	}

	wrong := newHarness(t)
	wrong.store = h.store
	wrong.app.readSecret = func(string) ([]byte, error) { return []byte("nope"), nil }
	if _, errs := wrong.mustFail(1, "key", "restore"); !strings.Contains(errs, "wrong passphrase") {
		t.Fatalf("key restore wrong passphrase: %q", errs)
	}
}

func TestKeyPassphraseRewrapsTheKey(t *testing.T) {
	h := newHarness(t)
	h.mustRun("init", "-N")
	live := h.write(".bashrc", "x\n", 0o644)
	h.mustRun("add", live)
	gen := h.generation()

	h.app.readSecret = func(string) ([]byte, error) { return []byte("new phrase"), nil }
	if out := h.mustRun("key", "passphrase"); !strings.Contains(out, "recovery passphrase changed") {
		t.Fatalf("key passphrase: %q", out)
	}
	if h.generation() != gen+1 {
		t.Fatalf("generation %d, want %d", h.generation(), gen+1)
	}
	if rec := h.lastSave(); rec.Action != "key passphrase" || rec.Generation != gen+1 {
		t.Fatalf("record after key passphrase: %+v", rec)
	}
	file, err := os.ReadFile(h.store)
	if err != nil {
		t.Fatal(err)
	}
	hdr, err := lockbox.ParseHeader(file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hdr.UnwrapKey([]byte("pw")); err == nil {
		t.Fatal("old passphrase still unwraps the key")
	}
	key, err := hdr.UnwrapKey([]byte("new phrase"))
	if err != nil {
		t.Fatal("new passphrase does not unwrap the key")
	}
	st, err := lockbox.Load(h.store, key)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := st.Entries()
	st.Close()
	if len(entries) != 1 || entries[0].Target != "~/.bashrc" {
		t.Fatalf("entries after rewrap: %+v", entries)
	}

	h.app.readLine = func(string) (string, error) { return "y", nil }
	h.mustRun("key", "rm")
	if _, errs := h.mustFail(1, "key", "passphrase"); !strings.Contains(errs, "key restore") {
		t.Fatalf("key passphrase without key: %q", errs)
	}
	h.app.isTerminal = func() bool { return false }
	if _, errs := h.mustFail(1, "key", "passphrase"); !strings.Contains(errs, "needs a terminal") {
		t.Fatalf("key passphrase without terminal: %q", errs)
	}
}

func TestKeyUsageErrors(t *testing.T) {
	h := newHarness(t)
	for _, args := range [][]string{{"key"}, {"key", "bogus"}, {"key", "show", "extra"}, {"key", "rm", "--nope"}} {
		if code, _, _ := h.run(args...); code != 2 {
			t.Fatalf("%v: code %d, want 2", args, code)
		}
	}
	if _, errs := h.mustFail(1, "key", "show"); !strings.Contains(errs, "") {
		t.Fatalf("key show without store: %q", errs)
	}
	if out, _ := h.mustFail(1, "key", "show"); !strings.Contains(out, "store file: no store at") {
		t.Fatalf("key show without store: %q", out)
	}
}
