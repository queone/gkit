package lockbox

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	_ "modernc.org/sqlite" // registers the pure-Go "sqlite" driver
)

const schemaVersion = 2

const schemaSQL = `
create table if not exists meta(
	key text primary key,
	value text not null
);
create table if not exists entries(
	id integer primary key,
	target text not null,
	mode integer not null,
	host text not null default '',
	uid integer,
	gid integer,
	owner text,
	grp text,
	unique(target, host)
);
create table if not exists versions(
	id integer primary key,
	entry_id integer not null references entries(id) on delete cascade,
	generation integer not null,
	sha256 text not null,
	content blob not null,
	captured_at text not null,
	captured_on text not null
);
create index if not exists versions_by_entry on versions(entry_id, generation);
`

// ErrExists reports a target already registered for the same host.
var ErrExists = errors.New("target already registered for this host")

// ErrSchema reports a store whose schema this build does not understand.
var ErrSchema = errors.New("unsupported store schema")

type serializer interface {
	Serialize() ([]byte, error)
	Deserialize([]byte) error
}

var bg = context.Background()

// Store is an open, in-memory copy of a sealed store file. Every read and
// write goes through one dedicated connection, because loading the database
// image detaches every other pooled connection.
type Store struct {
	Path   string
	Header Header
	key    []byte
	db     *sql.DB
	conn   *sql.Conn
}

// Version is one captured copy of an entry's content.
type Version struct {
	ID         int64
	Generation int64
	SHA256     string
	Content    []byte
	CapturedAt time.Time
	CapturedOn string
}

func openMemory() (*sql.DB, *sql.Conn, error) {
	db, err := sql.Open("sqlite", "file::memory:")
	if err != nil {
		return nil, nil, err
	}
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(bg)
	if err != nil {
		db.Close()
		return nil, nil, err
	}
	return db, conn, nil
}

// Create builds a new, empty store for path with header h and data key.
// Nothing is written until Save.
func Create(path string, h Header, key []byte) (*Store, error) {
	db, conn, err := openMemory()
	if err != nil {
		return nil, err
	}
	s := &Store{Path: path, Header: h, key: key, db: db, conn: conn}
	if err := s.prepare(); err != nil {
		s.Close()
		return nil, err
	}
	_, err = s.conn.ExecContext(bg,
		"insert into meta(key, value) values ('schema_version', ?), ('created_at', ?), ('key_id', ?)",
		strconv.Itoa(schemaVersion), time.Now().UTC().Format(time.RFC3339), KeyIDString(h.KeyID))
	if err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// Load opens the sealed store at path with key and loads it into memory.
func Load(path string, key []byte) (*Store, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	h, plain, err := Open(file, key)
	if err != nil {
		return nil, err
	}
	db, conn, err := openMemory()
	if err != nil {
		return nil, err
	}
	s := &Store{Path: path, Header: h, key: key, db: db, conn: conn}
	if err := s.conn.Raw(func(dc any) error { return dc.(serializer).Deserialize(plain) }); err != nil {
		s.Close()
		return nil, fmt.Errorf("load database image: %w", err)
	}
	if err := s.prepare(); err != nil {
		s.Close()
		return nil, err
	}
	var v string
	if err := s.conn.QueryRowContext(bg, "select value from meta where key = 'schema_version'").Scan(&v); err != nil {
		s.Close()
		return nil, ErrSchema
	}
	switch v {
	case "1":
		if err := s.migrateV1(); err != nil {
			s.Close()
			return nil, fmt.Errorf("migrate store schema: %w", err)
		}
	case strconv.Itoa(schemaVersion):
	default:
		s.Close()
		return nil, fmt.Errorf("%w: version %s", ErrSchema, v)
	}
	return s, nil
}

// migrateV1 upgrades a version-1 image in memory: entries gain the
// ownership columns. The next Save writes the upgraded image back.
func (s *Store) migrateV1() error {
	for _, stmt := range []string{
		"alter table entries add column uid integer",
		"alter table entries add column gid integer",
		"alter table entries add column owner text",
		"alter table entries add column grp text",
		"update meta set value = '2' where key = 'schema_version'",
	} {
		if _, err := s.conn.ExecContext(bg, stmt); err != nil {
			return err
		}
	}
	return nil
}

// SchemaVersion reports the schema version of the loaded image.
func (s *Store) SchemaVersion() (int, error) {
	var v string
	if err := s.conn.QueryRowContext(bg, "select value from meta where key = 'schema_version'").Scan(&v); err != nil {
		return 0, err
	}
	return strconv.Atoi(v)
}

func (s *Store) prepare() error {
	if _, err := s.conn.ExecContext(bg, "pragma foreign_keys = on"); err != nil {
		return err
	}
	_, err := s.conn.ExecContext(bg, schemaSQL)
	return err
}

// Save seals the database image and writes it to Path atomically. The
// generation advances by one on success.
func (s *Store) Save() error {
	var plain []byte
	if err := s.conn.Raw(func(dc any) error {
		var e error
		plain, e = dc.(serializer).Serialize()
		return e
	}); err != nil {
		return fmt.Errorf("serialize database: %w", err)
	}
	next := s.Header
	next.Generation = s.Header.Generation + 1
	file, err := Seal(next, s.key, plain)
	if err != nil {
		return err
	}
	if err := WriteAtomic(s.Path, file, s.Header.Generation); err != nil {
		return err
	}
	s.Header = next
	return nil
}

// Close releases the in-memory database without writing anything.
func (s *Store) Close() error {
	var first error
	if s.conn != nil {
		first = s.conn.Close()
		s.conn = nil
	}
	if s.db != nil {
		if err := s.db.Close(); first == nil {
			first = err
		}
		s.db = nil
	}
	return first
}

// Entries lists every registered entry ordered by target, then host.
func (s *Store) Entries() ([]Entry, error) {
	rows, err := s.conn.QueryContext(bg, "select id, target, mode, host, uid, gid, owner, grp from entries order by target, host")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		var mode int64
		var uid, gid sql.NullInt64
		var owner, group sql.NullString
		if err := rows.Scan(&e.ID, &e.Target, &mode, &e.Host, &uid, &gid, &owner, &group); err != nil {
			return nil, err
		}
		e.Mode = os.FileMode(mode)
		e.Ownership = UnknownOwnership
		if uid.Valid && gid.Valid {
			e.Ownership = Ownership{UID: uid.Int64, GID: gid.Int64, Owner: owner.String, Group: group.String}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) hasEntry(target, host string) (bool, error) {
	var n int
	if err := s.conn.QueryRowContext(bg, "select count(*) from entries where target = ? and host = ?", target, host).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// AddEntry registers target with mode, host ("" for every Mac), and
// ownership. It returns ErrExists when that target and host pair is already
// registered.
func (s *Store) AddEntry(target string, mode os.FileMode, host string, own Ownership) (Entry, error) {
	exists, err := s.hasEntry(target, host)
	if err != nil {
		return Entry{}, err
	}
	if exists {
		return Entry{}, ErrExists
	}
	args := append([]any{target, int64(mode.Perm()), host}, ownerArgs(own)...)
	res, err := s.conn.ExecContext(bg, "insert into entries(target, mode, host, uid, gid, owner, grp) values (?, ?, ?, ?, ?, ?, ?)", args...)
	if err != nil {
		return Entry{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Entry{}, err
	}
	if !own.Known() {
		own = UnknownOwnership
	}
	return Entry{ID: id, Target: target, Mode: mode.Perm(), Host: host, Ownership: own}, nil
}

// ownerArgs renders ownership as SQL arguments, NULL when unknown.
func ownerArgs(own Ownership) []any {
	if !own.Known() {
		return []any{nil, nil, nil, nil}
	}
	return []any{own.UID, own.GID, own.Owner, own.Group}
}

// SetOwner records new ownership for an entry.
func (s *Store) SetOwner(id int64, own Ownership) error {
	args := append(ownerArgs(own), id)
	_, err := s.conn.ExecContext(bg, "update entries set uid = ?, gid = ?, owner = ?, grp = ? where id = ?", args...)
	return err
}

// SetHost rebinds an entry to host ("" for every Mac). It returns ErrExists
// when another entry already holds that target and host.
func (s *Store) SetHost(id int64, host string) error {
	var target string
	if err := s.conn.QueryRowContext(bg, "select target from entries where id = ?", id).Scan(&target); err != nil {
		return err
	}
	exists, err := s.hasEntry(target, host)
	if err != nil {
		return err
	}
	if exists {
		return ErrExists
	}
	_, err = s.conn.ExecContext(bg, "update entries set host = ? where id = ?", host, id)
	return err
}

// RemoveEntry deletes an entry and every version it holds.
func (s *Store) RemoveEntry(id int64) error {
	res, err := s.conn.ExecContext(bg, "delete from entries where id = ?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("entry not found")
	}
	return nil
}

// SetMode records a new file mode for an entry.
func (s *Store) SetMode(id int64, mode os.FileMode) error {
	_, err := s.conn.ExecContext(bg, "update entries set mode = ? where id = ?", int64(mode.Perm()), id)
	return err
}

// Latest returns the newest version of an entry. ok is false when the entry
// holds no version yet.
func (s *Store) Latest(entryID int64) (v Version, ok bool, err error) {
	var at string
	err = s.conn.QueryRowContext(bg,
		"select id, generation, sha256, content, captured_at, captured_on from versions where entry_id = ? order by generation desc limit 1",
		entryID).Scan(&v.ID, &v.Generation, &v.SHA256, &v.Content, &at, &v.CapturedOn)
	if errors.Is(err, sql.ErrNoRows) {
		return Version{}, false, nil
	}
	if err != nil {
		return Version{}, false, err
	}
	v.CapturedAt, _ = time.Parse(time.RFC3339, at)
	return v, true, nil
}

// Versions lists every stored version of an entry, oldest first.
func (s *Store) Versions(entryID int64) ([]Version, error) {
	rows, err := s.conn.QueryContext(bg,
		"select id, generation, sha256, content, captured_at, captured_on from versions where entry_id = ? order by generation",
		entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Version
	for rows.Next() {
		var v Version
		var at string
		if err := rows.Scan(&v.ID, &v.Generation, &v.SHA256, &v.Content, &at, &v.CapturedOn); err != nil {
			return nil, err
		}
		v.CapturedAt, _ = time.Parse(time.RFC3339, at)
		out = append(out, v)
	}
	return out, rows.Err()
}

// VersionCount returns how many versions an entry holds.
func (s *Store) VersionCount(entryID int64) (int, error) {
	var n int
	err := s.conn.QueryRowContext(bg, "select count(*) from versions where entry_id = ?", entryID).Scan(&n)
	return n, err
}

// AddVersion records content as the next version of an entry, captured on
// host capturedOn.
func (s *Store) AddVersion(entryID int64, content []byte, capturedOn string) (Version, error) {
	if content == nil {
		content = []byte{} // an empty file is an empty blob, never NULL
	}
	var gen int64
	if err := s.conn.QueryRowContext(bg, "select coalesce(max(generation), 0) + 1 from versions where entry_id = ?", entryID).Scan(&gen); err != nil {
		return Version{}, err
	}
	v := Version{
		Generation: gen,
		SHA256:     Digest(content),
		Content:    content,
		CapturedAt: time.Now().UTC().Truncate(time.Second),
		CapturedOn: capturedOn,
	}
	res, err := s.conn.ExecContext(bg,
		"insert into versions(entry_id, generation, sha256, content, captured_at, captured_on) values (?, ?, ?, ?, ?, ?)",
		entryID, v.Generation, v.SHA256, v.Content, v.CapturedAt.Format(time.RFC3339), v.CapturedOn)
	if err != nil {
		return Version{}, err
	}
	v.ID, err = res.LastInsertId()
	return v, err
}

// Digest returns the hex SHA-256 of content, the identity used to detect drift.
func Digest(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
