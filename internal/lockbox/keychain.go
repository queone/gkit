package lockbox

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrKeyNotFound reports a keychain that holds no key for the store.
var ErrKeyNotFound = errors.New("key not found in keychain")

// KeyStore stores and retrieves a store's data key by key id.
type KeyStore interface {
	Get(keyID string) ([]byte, error)
	Put(keyID string, key []byte) error
	Delete(keyID string) error
}

// Executor runs an external command and returns its combined output.
type Executor func(name string, args ...string) ([]byte, error)

// DefaultExecutor runs commands through os/exec.
func DefaultExecutor(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// defaultKeychainService names the keychain service used when a
// SecurityKeyStore leaves Service empty.
const defaultKeychainService = "macfit"

// SecurityKeyStore keeps the data key in the login keychain through the
// macOS security command. Items the command creates are readable by the
// command without an access prompt.
type SecurityKeyStore struct {
	Exec Executor
	// Service is the keychain service name the items belong to, so each
	// tool keeps its own keys apart. Empty means macfit.
	Service string
}

// service returns the keychain service name, macfit when none is set.
func (s SecurityKeyStore) service() string {
	if s.Service == "" {
		return defaultKeychainService
	}
	return s.Service
}

func (s SecurityKeyStore) run(args ...string) ([]byte, error) {
	if s.Exec == nil {
		return DefaultExecutor("security", args...)
	}
	return s.Exec("security", args...)
}

// Get reads the key for keyID from the login keychain.
func (s SecurityKeyStore) Get(keyID string) ([]byte, error) {
	out, err := s.run("find-generic-password", "-a", keyID, "-s", s.service(), "-w")
	if err != nil {
		if bytes.Contains(out, []byte("could not be found")) {
			return nil, ErrKeyNotFound
		}
		return nil, fmt.Errorf("read keychain item: %w: %s", err, strings.TrimSpace(string(out)))
	}
	key, err := hex.DecodeString(strings.TrimSpace(string(out)))
	if err != nil || len(key) != KeySize {
		return nil, fmt.Errorf("keychain item is not a %s key", s.service())
	}
	return key, nil
}

// Put saves key for keyID in the login keychain, replacing any existing item.
func (s SecurityKeyStore) Put(keyID string, key []byte) error {
	if len(key) != KeySize {
		return fmt.Errorf("data key must be %d bytes", KeySize)
	}
	out, err := s.run("add-generic-password", "-a", keyID, "-s", s.service(), "-w", hex.EncodeToString(key), "-U")
	if err != nil {
		return fmt.Errorf("save keychain item: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Delete removes the keychain item for keyID. It returns ErrKeyNotFound
// when no item exists.
func (s SecurityKeyStore) Delete(keyID string) error {
	out, err := s.run("delete-generic-password", "-a", keyID, "-s", s.service())
	if err != nil {
		if bytes.Contains(out, []byte("could not be found")) {
			return ErrKeyNotFound
		}
		return fmt.Errorf("delete keychain item: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// MemoryKeyStore is an in-memory KeyStore for tests.
type MemoryKeyStore struct {
	Keys map[string][]byte
}

// Get returns the key saved for keyID.
func (m *MemoryKeyStore) Get(keyID string) ([]byte, error) {
	if k, ok := m.Keys[keyID]; ok {
		return bytes.Clone(k), nil
	}
	return nil, ErrKeyNotFound
}

// Put saves key for keyID.
func (m *MemoryKeyStore) Put(keyID string, key []byte) error {
	if m.Keys == nil {
		m.Keys = map[string][]byte{}
	}
	m.Keys[keyID] = bytes.Clone(key)
	return nil
}

// Delete removes the key saved for keyID.
func (m *MemoryKeyStore) Delete(keyID string) error {
	if _, ok := m.Keys[keyID]; !ok {
		return ErrKeyNotFound
	}
	delete(m.Keys, keyID)
	return nil
}
