// Package lockbox seals a SQLite database inside an authenticated encryption
// envelope, keeps the data key in the macOS login keychain, and maps store
// entries to live paths.
package lockbox

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

const (
	// formatTag opens every store file. The bytes come from macfit, the tool
	// that created the format, and must not change, or every existing store
	// becomes unreadable.
	formatTag     = "MACFIT"
	formatVersion = 1
	// KeySize is the data key length in bytes.
	KeySize     = chacha20poly1305.KeySize
	keyIDSize   = 16
	saltSize    = 16
	nonceSize   = chacha20poly1305.NonceSizeX
	wrappedSize = nonceSize + KeySize + chacha20poly1305.Overhead
	headerSize  = len(formatTag) + 1 + 8 + keyIDSize + saltSize + 4 + 4 + 1 + wrappedSize + nonceSize
)

// KDF holds the Argon2id parameters that derive the wrapping key from the
// recovery passphrase.
type KDF struct {
	Time    uint32
	Memory  uint32 // KiB
	Threads uint8
}

// DefaultKDF is the parameter set used for new stores.
var DefaultKDF = KDF{Time: 3, Memory: 64 * 1024, Threads: 4}

// Header is the unencrypted, authenticated part of a store file.
type Header struct {
	Generation uint64
	KeyID      [keyIDSize]byte
	Salt       [saltSize]byte
	KDF        KDF
	Wrapped    [wrappedSize]byte
	nonce      [nonceSize]byte
}

var (
	// ErrFormat reports a file that is not an encrypted store.
	ErrFormat = errors.New("not an encrypted store")
	// ErrAuth reports a wrong data key or a damaged file.
	ErrAuth = errors.New("store authentication failed: wrong key or damaged file")
	// ErrPassphrase reports a recovery passphrase that does not unwrap the key.
	ErrPassphrase = errors.New("wrong passphrase")
	// ErrConflict reports a store file that changed on disk after it was opened.
	ErrConflict = errors.New("store changed on disk since it was opened")
)

// NewKey draws a fresh random data key.
func NewKey() ([]byte, error) {
	k := make([]byte, KeySize)
	if _, err := rand.Read(k); err != nil {
		return nil, err
	}
	return k, nil
}

// NewKeyID draws a fresh random key id.
func NewKeyID() (id [keyIDSize]byte, err error) {
	_, err = rand.Read(id[:])
	return id, err
}

// KeyIDString formats a key id for keychain lookups.
func KeyIDString(id [keyIDSize]byte) string { return hex.EncodeToString(id[:]) }

func deriveWrapKey(passphrase []byte, salt [saltSize]byte, kdf KDF) []byte {
	return argon2.IDKey(passphrase, salt[:], kdf.Time, kdf.Memory, kdf.Threads, KeySize)
}

// WrapKey seals key under a key derived from passphrase and fills Salt, KDF,
// and Wrapped. KeyID must be set first because it authenticates the wrap.
func (h *Header) WrapKey(key, passphrase []byte, kdf KDF) error {
	if len(key) != KeySize {
		return fmt.Errorf("data key must be %d bytes", KeySize)
	}
	if kdf.Time == 0 || kdf.Memory == 0 || kdf.Threads == 0 {
		return errors.New("invalid passphrase derivation parameters")
	}
	if _, err := rand.Read(h.Salt[:]); err != nil {
		return err
	}
	h.KDF = kdf
	aead, err := chacha20poly1305.NewX(deriveWrapKey(passphrase, h.Salt, kdf))
	if err != nil {
		return err
	}
	var nonce [nonceSize]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	out := make([]byte, 0, wrappedSize)
	out = append(out, nonce[:]...)
	out = aead.Seal(out, nonce[:], key, h.KeyID[:])
	copy(h.Wrapped[:], out)
	return nil
}

// UnwrapKey recovers the data key from Wrapped with the recovery passphrase.
func (h Header) UnwrapKey(passphrase []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(deriveWrapKey(passphrase, h.Salt, h.KDF))
	if err != nil {
		return nil, err
	}
	key, err := aead.Open(nil, h.Wrapped[:nonceSize], h.Wrapped[nonceSize:], h.KeyID[:])
	if err != nil {
		return nil, ErrPassphrase
	}
	return key, nil
}

func (h Header) encode() []byte {
	b := make([]byte, 0, headerSize)
	b = append(b, formatTag...)
	b = append(b, formatVersion)
	b = binary.BigEndian.AppendUint64(b, h.Generation)
	b = append(b, h.KeyID[:]...)
	b = append(b, h.Salt[:]...)
	b = binary.BigEndian.AppendUint32(b, h.KDF.Time)
	b = binary.BigEndian.AppendUint32(b, h.KDF.Memory)
	b = append(b, h.KDF.Threads)
	b = append(b, h.Wrapped[:]...)
	b = append(b, h.nonce[:]...)
	return b
}

// ParseHeader reads the header of a store file. No key is needed.
func ParseHeader(file []byte) (Header, error) {
	var h Header
	if len(file) < headerSize || string(file[:len(formatTag)]) != formatTag {
		return h, ErrFormat
	}
	p := len(formatTag)
	if file[p] != formatVersion {
		return h, fmt.Errorf("%w: unsupported format version %d", ErrFormat, file[p])
	}
	p++
	h.Generation = binary.BigEndian.Uint64(file[p:])
	p += 8
	copy(h.KeyID[:], file[p:])
	p += keyIDSize
	copy(h.Salt[:], file[p:])
	p += saltSize
	h.KDF.Time = binary.BigEndian.Uint32(file[p:])
	p += 4
	h.KDF.Memory = binary.BigEndian.Uint32(file[p:])
	p += 4
	h.KDF.Threads = file[p]
	p++
	copy(h.Wrapped[:], file[p:])
	p += wrappedSize
	copy(h.nonce[:], file[p:])
	return h, nil
}

// Seal encrypts plain under key and returns the complete file bytes. The
// Generation, KeyID, Salt, KDF, and Wrapped fields come from h; a fresh body
// nonce is drawn for every call.
func Seal(h Header, key, plain []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	if _, err := rand.Read(h.nonce[:]); err != nil {
		return nil, err
	}
	hdr := h.encode()
	out := make([]byte, 0, len(hdr)+len(plain)+aead.Overhead())
	out = append(out, hdr...)
	return aead.Seal(out, h.nonce[:], plain, hdr), nil
}

// Open authenticates file with key and returns its header and plaintext.
func Open(file, key []byte) (Header, []byte, error) {
	h, err := ParseHeader(file)
	if err != nil {
		return h, nil, err
	}
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return h, nil, err
	}
	plain, err := aead.Open(nil, h.nonce[:], file[headerSize:], file[:headerSize])
	if err != nil {
		return h, nil, ErrAuth
	}
	return h, plain, nil
}

// renameFile is swapped by tests to inject a rename failure.
var renameFile = os.Rename

// WriteAtomic writes data to path through a temporary sibling and a rename.
// It refuses with ErrConflict when the file on disk no longer carries
// generation openedGen. Pass 0 for a store that must not exist yet.
func WriteAtomic(path string, data []byte, openedGen uint64) error {
	current, err := os.ReadFile(path)
	switch {
	case err == nil:
		h, perr := ParseHeader(current)
		if perr != nil {
			return perr
		}
		if h.Generation != openedGen {
			return ErrConflict
		}
	case errors.Is(err, os.ErrNotExist):
		if openedGen != 0 {
			return ErrConflict
		}
	default:
		return err
	}
	dir, base := filepath.Dir(path), filepath.Base(path)
	tmp, err := os.CreateTemp(dir, "."+base+".tmp-")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	fail := func(e error) error {
		tmp.Close()
		os.Remove(tmpPath)
		return e
	}
	if _, err := tmp.Write(data); err != nil {
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := renameFile(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

// ConflictCopies lists sync conflict copies beside path: every sibling that
// starts with the store's stem, ends with its extension, and carries a suffix
// between them that starts with a space, an opening parenthesis, or a hyphen,
// the shapes sync clients produce: "vault 2.store", "vault (1).store",
// "vault (conflicted copy 2026-09-09).store", "vault-DESKTOP.store".
func ConflictCopies(path string) []string {
	dir, base := filepath.Dir(path), filepath.Base(path)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		n := e.Name()
		if n == base || !strings.HasPrefix(n, stem) || !strings.HasSuffix(n, ext) {
			continue
		}
		mid := n[len(stem) : len(n)-len(ext)]
		if mid == "" || !strings.ContainsRune(" (-", rune(mid[0])) {
			continue
		}
		out = append(out, filepath.Join(dir, n))
	}
	return out
}
