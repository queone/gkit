package lockbox

import (
	"bytes"
	"encoding/hex"
	"errors"
	"reflect"
	"testing"
)

func TestSecurityKeyStoreBuildsExpectedArguments(t *testing.T) {
	key := bytes.Repeat([]byte{0xab}, KeySize)
	var calls [][]string
	ks := SecurityKeyStore{Exec: func(name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		if args[0] == "find-generic-password" {
			return []byte(hex.EncodeToString(key) + "\n"), nil
		}
		return nil, nil
	}}
	if err := ks.Put("abc123", key); err != nil {
		t.Fatal(err)
	}
	got, err := ks.Get("abc123")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, key) {
		t.Fatal("key read back differs")
	}
	if err := ks.Delete("abc123"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"security", "add-generic-password", "-a", "abc123", "-s", "macfit", "-w", hex.EncodeToString(key), "-U"},
		{"security", "find-generic-password", "-a", "abc123", "-s", "macfit", "-w"},
		{"security", "delete-generic-password", "-a", "abc123", "-s", "macfit"},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("security calls\n got %v\nwant %v", calls, want)
	}
}

func TestSecurityKeyStoreMapsMissingItem(t *testing.T) {
	ks := SecurityKeyStore{Exec: func(string, ...string) ([]byte, error) {
		return []byte("security: SecKeychainSearchCopyNext: The specified item could not be found in the keychain.\n"), errors.New("exit status 44")
	}}
	if _, err := ks.Get("abc"); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("got %v, want ErrKeyNotFound", err)
	}
	if err := ks.Delete("abc"); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("delete missing: got %v, want ErrKeyNotFound", err)
	}
	broken := SecurityKeyStore{Exec: func(string, ...string) ([]byte, error) { return []byte("zz\n"), nil }}
	if _, err := broken.Get("abc"); err == nil {
		t.Fatal("malformed keychain item was accepted")
	}
	failing := SecurityKeyStore{Exec: func(string, ...string) ([]byte, error) { return []byte("locked"), errors.New("exit status 1") }}
	if _, err := failing.Get("abc"); err == nil || errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("other failure reported as %v", err)
	}
}

func TestMemoryKeyStore(t *testing.T) {
	var m MemoryKeyStore
	if _, err := m.Get("x"); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("empty store: got %v", err)
	}
	key := bytes.Repeat([]byte{1}, KeySize)
	if err := m.Put("x", key); err != nil {
		t.Fatal(err)
	}
	got, err := m.Get("x")
	if err != nil || !bytes.Equal(got, key) {
		t.Fatalf("get after put: %v %v", got, err)
	}
	if err := m.Delete("x"); err != nil {
		t.Fatal(err)
	}
	if err := m.Delete("x"); !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("second delete: got %v", err)
	}
}

func TestSecurityKeyStoreUsesItsOwnServiceName(t *testing.T) {
	key := bytes.Repeat([]byte{0xcd}, KeySize)
	var calls [][]string
	ks := SecurityKeyStore{Service: "attune", Exec: func(name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		if args[0] == "find-generic-password" {
			return []byte("zz\n"), nil
		}
		return nil, nil
	}}
	if err := ks.Put("k1", key); err != nil {
		t.Fatal(err)
	}
	_, err := ks.Get("k1")
	if err == nil || err.Error() != "keychain item is not a attune key" {
		t.Fatalf("malformed item error %v, want the service name in it", err)
	}
	if err := ks.Delete("k1"); err != nil {
		t.Fatal(err)
	}
	for _, call := range calls {
		if call[5] != "attune" {
			t.Fatalf("call %v does not use the attune service", call)
		}
	}
	if (SecurityKeyStore{}).service() != "macfit" {
		t.Fatal("an empty Service must mean macfit")
	}
}
