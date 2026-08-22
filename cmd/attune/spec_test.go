package main

import "testing"

func TestFixtureLoadsEveryKind(t *testing.T) {
	bundle, err := Load("testdata/specs")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if bundle.Len() != 6 {
		t.Errorf("bundle.Len() = %d, want 6", bundle.Len())
	}
}

func TestDirectoryObjectIdRequiresExactUuidShape(t *testing.T) {
	valid := []string{
		"00000000-0000-0000-0000-000000000001",
		"ABCDEF00-1234-5678-9ABC-DEF012345678",
	}
	for _, v := range valid {
		if !isDirectoryObjectID(v) {
			t.Errorf("isDirectoryObjectID(%q) = false, want true", v)
		}
	}
	invalid := []string{
		"synthetic-principal",
		"000000000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-00000000000z",
	}
	for _, v := range invalid {
		if isDirectoryObjectID(v) {
			t.Errorf("isDirectoryObjectID(%q) = true, want false", v)
		}
	}
}
