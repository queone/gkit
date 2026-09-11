package lockbox

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T, dir string) (*Store, []byte) {
	t.Helper()
	key := mustKey(t)
	h := newTestHeader(t, key, "pw")
	h.Generation = 0
	st, err := Create(filepath.Join(dir, "macfit.store"), h, key)
	if err != nil {
		t.Fatal(err)
	}
	return st, key
}

func TestStoreLifecycleWritesOnlyTheStoreFile(t *testing.T) {
	dir := t.TempDir()
	st, key := newTestStore(t, dir)
	e, err := st.AddEntry("~/.bashrc", 0o644, "", Ownership{UID: 501, GID: 20, Owner: "tek1", Group: "staff"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddVersion(e.ID, []byte("export X=1\n"), "np10"); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
	if st.Header.Generation != 1 {
		t.Fatalf("generation after first save %d, want 1", st.Header.Generation)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	names := dirNames(t, dir)
	if len(names) != 1 || names[0] != "macfit.store" {
		t.Fatalf("directory holds %v, want only macfit.store", names)
	}
	info, err := os.Stat(filepath.Join(dir, "macfit.store"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("store mode %o, want 600", info.Mode().Perm())
	}

	re, err := Load(filepath.Join(dir, "macfit.store"), key)
	if err != nil {
		t.Fatal(err)
	}
	defer re.Close()
	entries, err := re.Entries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Target != "~/.bashrc" || entries[0].Mode != 0o644 || entries[0].Host != "" {
		t.Fatalf("entries after reload: %+v", entries)
	}
	if want := (Ownership{UID: 501, GID: 20, Owner: "tek1", Group: "staff"}); entries[0].Ownership != want {
		t.Fatalf("ownership after reload %+v, want %+v", entries[0].Ownership, want)
	}
	if v, err := re.SchemaVersion(); err != nil || v != 2 {
		t.Fatalf("schema version %d %v", v, err)
	}
	v, ok, err := re.Latest(entries[0].ID)
	if err != nil || !ok {
		t.Fatalf("latest: ok=%v err=%v", ok, err)
	}
	if string(v.Content) != "export X=1\n" || v.CapturedOn != "np10" || v.Generation != 1 || v.CapturedAt.IsZero() {
		t.Fatalf("version after reload: %+v", v)
	}
	if re.Header.Generation != 1 {
		t.Fatalf("reloaded generation %d, want 1", re.Header.Generation)
	}
}

func TestStoreVersionsDuplicatesAndCascade(t *testing.T) {
	dir := t.TempDir()
	st, _ := newTestStore(t, dir)
	defer st.Close()
	e, err := st.AddEntry("$XDG_CONFIG_HOME/git/config", 0o600, "", UnknownOwnership)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddEntry("$XDG_CONFIG_HOME/git/config", 0o600, "", UnknownOwnership); !errors.Is(err, ErrExists) {
		t.Fatalf("duplicate unbound entry: got %v, want ErrExists", err)
	}
	if _, err := st.AddEntry("$XDG_CONFIG_HOME/git/config", 0o600, "np10", UnknownOwnership); err != nil {
		t.Fatalf("host-bound entry for the same target must be allowed: %v", err)
	}
	if _, ok, err := st.Latest(e.ID); err != nil || ok {
		t.Fatalf("latest on empty entry: ok=%v err=%v", ok, err)
	}
	v1, err := st.AddVersion(e.ID, []byte("one"), "a")
	if err != nil {
		t.Fatal(err)
	}
	v2, err := st.AddVersion(e.ID, []byte("two"), "a")
	if err != nil {
		t.Fatal(err)
	}
	if v1.Generation != 1 || v2.Generation != 2 || v1.SHA256 == v2.SHA256 {
		t.Fatalf("versions: %+v %+v", v1, v2)
	}
	latest, _, _ := st.Latest(e.ID)
	if !bytes.Equal(latest.Content, []byte("two")) {
		t.Fatalf("latest content %q", latest.Content)
	}
	all, err := st.Versions(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].Generation != 1 || string(all[0].Content) != "one" || all[1].Generation != 2 || string(all[1].Content) != "two" || all[1].CapturedAt.IsZero() {
		t.Fatalf("versions listing: %+v", all)
	}
	if none, err := st.Versions(9999); err != nil || len(none) != 0 {
		t.Fatalf("versions of a missing entry: %v %v", none, err)
	}
	if err := st.SetMode(e.ID, 0o644); err != nil {
		t.Fatal(err)
	}
	entries, _ := st.Entries()
	if entries[0].Mode != 0o644 {
		t.Fatalf("mode after SetMode %o", entries[0].Mode)
	}
	if entries[0].Known() {
		t.Fatalf("unknown ownership must not read as known: %+v", entries[0].Ownership)
	}
	if err := st.SetOwner(e.ID, Ownership{UID: 1, GID: 2, Owner: "u", Group: "g"}); err != nil {
		t.Fatal(err)
	}
	entries, _ = st.Entries()
	if !entries[0].Known() || entries[0].Owner != "u" || entries[0].Group != "g" {
		t.Fatalf("ownership after SetOwner %+v", entries[0].Ownership)
	}
	if err := st.SetHost(e.ID, "np10"); !errors.Is(err, ErrExists) {
		t.Fatalf("SetHost onto an existing binding: got %v, want ErrExists", err)
	}
	if err := st.SetHost(e.ID, "np11"); err != nil {
		t.Fatal(err)
	}
	entries, _ = st.Entries()
	if entries[0].Host != "np10" || entries[1].Host != "np11" {
		t.Fatalf("hosts after SetHost: %+v", entries)
	}
	if n, _ := st.VersionCount(e.ID); n != 2 {
		t.Fatalf("SetHost lost versions: %d", n)
	}
	if err := st.RemoveEntry(e.ID); err != nil {
		t.Fatal(err)
	}
	if n, _ := st.VersionCount(e.ID); n != 0 {
		t.Fatalf("versions survived entry removal: %d", n)
	}
	if err := st.RemoveEntry(e.ID); err == nil {
		t.Fatal("removing a missing entry must fail")
	}
}

func TestStoreSaveDetectsConcurrentWriter(t *testing.T) {
	dir := t.TempDir()
	st, key := newTestStore(t, dir)
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
	st.Close()
	path := filepath.Join(dir, "macfit.store")
	first, err := Load(path, key)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := Load(path, key)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if _, err := first.AddEntry("~/.a", 0o644, "", UnknownOwnership); err != nil {
		t.Fatal(err)
	}
	if err := first.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := second.AddEntry("~/.b", 0o644, "", UnknownOwnership); err != nil {
		t.Fatal(err)
	}
	if err := second.Save(); !errors.Is(err, ErrConflict) {
		t.Fatalf("second writer: got %v, want ErrConflict", err)
	}
	check, err := Load(path, key)
	if err != nil {
		t.Fatal(err)
	}
	defer check.Close()
	entries, _ := check.Entries()
	if len(entries) != 1 || entries[0].Target != "~/.a" {
		t.Fatalf("store on disk holds %+v, want only ~/.a", entries)
	}
}

func TestLoadRejectsWrongKeyAndForeignFile(t *testing.T) {
	dir := t.TempDir()
	st, _ := newTestStore(t, dir)
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
	st.Close()
	path := filepath.Join(dir, "macfit.store")
	if _, err := Load(path, mustKey(t)); !errors.Is(err, ErrAuth) {
		t.Fatalf("wrong key: got %v", err)
	}
	other := filepath.Join(dir, "plain.txt")
	os.WriteFile(other, []byte("not a store"), 0o600)
	if _, err := Load(other, mustKey(t)); !errors.Is(err, ErrFormat) {
		t.Fatalf("foreign file: got %v", err)
	}
}

// writeV1Store writes a store file in schema version 1 holding one entry
// and one version, the layout macfit v1.x wrote.
func writeV1Store(t *testing.T, path string, key []byte, h Header, version string) {
	t.Helper()
	db, conn, err := openMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	defer conn.Close()
	for _, stmt := range []string{
		"create table meta(key text primary key, value text not null)",
		"create table entries(id integer primary key, target text not null, mode integer not null, host text not null default '', unique(target, host))",
		"create table versions(id integer primary key, entry_id integer not null references entries(id) on delete cascade, generation integer not null, sha256 text not null, content blob not null, captured_at text not null, captured_on text not null)",
		"insert into meta(key, value) values ('schema_version', '" + version + "'), ('created_at', '2026-09-09T00:00:00Z'), ('key_id', 'x')",
		"insert into entries(target, mode, host) values ('~/.bashrc', 420, '')",
		"insert into versions(entry_id, generation, sha256, content, captured_at, captured_on) values (1, 1, 'd', X'6869', '2026-09-09T00:00:00Z', 'np10')",
	} {
		if _, err := conn.ExecContext(bg, stmt); err != nil {
			t.Fatal(err)
		}
	}
	var plain []byte
	if err := conn.Raw(func(dc any) error {
		var e error
		plain, e = dc.(serializer).Serialize()
		return e
	}); err != nil {
		t.Fatal(err)
	}
	file, err := Seal(h, key, plain)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(path, file, 0); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMigratesVersionOneInMemoryAndSaveWritesVersionTwo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "macfit.store")
	key := mustKey(t)
	h := newTestHeader(t, key, "pw")
	h.Generation = 1
	writeV1Store(t, path, key, h, "1")

	st, err := Load(path, key)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := st.Entries()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Target != "~/.bashrc" || entries[0].Known() {
		t.Fatalf("migrated entries: %+v", entries)
	}
	if v, _ := st.SchemaVersion(); v != 2 {
		t.Fatalf("in-memory schema version %d, want 2", v)
	}
	if v, ok, err := st.Latest(entries[0].ID); err != nil || !ok || string(v.Content) != "hi" {
		t.Fatalf("version after migration: %+v %v %v", v, ok, err)
	}
	st.Close()

	again, err := Load(path, key)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := again.SchemaVersion(); v != 2 {
		t.Fatal("a read-only load must still migrate in memory")
	}
	if err := again.Save(); err != nil {
		t.Fatal(err)
	}
	again.Close()

	saved, err := Load(path, key)
	if err != nil {
		t.Fatal(err)
	}
	defer saved.Close()
	var n int
	if err := saved.conn.QueryRowContext(bg, "select count(*) from pragma_table_info('entries') where name in ('uid', 'gid', 'owner', 'grp')").Scan(&n); err != nil || n != 4 {
		t.Fatalf("ownership columns after save: %d %v", n, err)
	}
	entries, _ = saved.Entries()
	if len(entries) != 1 || entries[0].Known() {
		t.Fatalf("entries after saved migration: %+v", entries)
	}

	future := filepath.Join(dir, "future.store")
	writeV1Store(t, future, key, h, "3")
	if _, err := Load(future, key); !errors.Is(err, ErrSchema) {
		t.Fatalf("version 3 store: got %v, want ErrSchema", err)
	}
}

func TestSetTargetRenamesAndRefusesCollision(t *testing.T) {
	st, _ := newTestStore(t, t.TempDir())
	defer st.Close()
	a, err := st.AddEntry("dns/a.yaml", 0o600, "", UnknownOwnership)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddVersion(a.ID, []byte("kind: dnsRecordSet\n"), "np10"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddEntry("dns/b.yaml", 0o600, "", UnknownOwnership); err != nil {
		t.Fatal(err)
	}
	if err := st.SetTarget(a.ID, "dns/b.yaml"); !errors.Is(err, ErrExists) {
		t.Fatalf("rename onto a held target: got %v, want ErrExists", err)
	}
	if err := st.SetTarget(a.ID, "zones/a.yaml"); err != nil {
		t.Fatal(err)
	}
	entries, err := st.Entries()
	if err != nil {
		t.Fatal(err)
	}
	targets := []string{}
	for _, e := range entries {
		targets = append(targets, e.Target)
	}
	if len(targets) != 2 || targets[0] != "dns/b.yaml" || targets[1] != "zones/a.yaml" {
		t.Fatalf("targets after rename %v", targets)
	}
	if n, err := st.VersionCount(a.ID); err != nil || n != 1 {
		t.Fatalf("versions after rename %d %v, want 1", n, err)
	}
	if err := st.SetTarget(999, "x.yaml"); err == nil {
		t.Fatal("renaming a missing entry succeeded")
	}
}
