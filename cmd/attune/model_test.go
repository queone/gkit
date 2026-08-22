package main

import "testing"

func TestScopeRequiresResourceGroupForDnsZone(t *testing.T) {
	scope := Scope{DnsZone: "example.com"}
	if _, err := scope.ArmID("00000000-0000-0000-0000-000000000001"); err == nil {
		t.Errorf("expected error for dnsZone scope without resourceGroup")
	}
}

func TestDnsValuesAreOrderInsensitive(t *testing.T) {
	left := DnsRecord{
		Zone:       "example.com",
		RecordType: "TXT",
		Name:       "@",
		TTL:        60,
		Values:     []string{"b", "a"},
	}
	right := left
	right.Values = []string{"a", "b"}
	if !left.SameData(right) {
		t.Errorf("expected order-insensitive DNS value comparison to match")
	}
}
