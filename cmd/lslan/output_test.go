package main

import (
	"bytes"
	"net"
	"testing"
	"time"
)

func sampleHosts() []host {
	return []host{
		{IP: net.IPv4(10, 0, 0, 2).To4(), RTT: 3 * time.Millisecond, Via: []string{"icmp", "tcp80"}, Name: "gw.lan", MAC: "aa:bb:cc:dd:ee:ff"},
		{IP: net.IPv4(10, 0, 0, 9).To4(), Via: []string{"self"}},
	}
}

func TestTSVHasFiveColumnsAndNoHeader(t *testing.T) {
	var out bytes.Buffer
	if err := writeTSV(&out, sampleHosts()); err != nil {
		t.Fatal(err)
	}
	want := "10.0.0.2\t3\ticmp,tcp80\tgw.lan\taa:bb:cc:dd:ee:ff\n10.0.0.9\t0\tself\t\t\n"
	if out.String() != want {
		t.Errorf("tsv = %q, want %q", out.String(), want)
	}
	for line := range bytes.SplitSeq(bytes.TrimSuffix(out.Bytes(), []byte("\n")), []byte("\n")) {
		if n := bytes.Count(line, []byte("\t")); n != 4 {
			t.Errorf("line %q has %d tabs, want 4", line, n)
		}
	}
}

func TestJSONCarriesTheFiveKeys(t *testing.T) {
	var out bytes.Buffer
	if err := writeJSON(&out, sampleHosts()); err != nil {
		t.Fatal(err)
	}
	want := `[{"ip":"10.0.0.2","rtt_ms":3,"via":["icmp","tcp80"],"name":"gw.lan","mac":"aa:bb:cc:dd:ee:ff"},` +
		`{"ip":"10.0.0.9","rtt_ms":0,"via":["self"],"name":"","mac":""}]` + "\n"
	if out.String() != want {
		t.Errorf("json = %q, want %q", out.String(), want)
	}
	var empty bytes.Buffer
	if err := writeJSON(&empty, nil); err != nil || empty.String() != "[]\n" {
		t.Errorf("empty json = %q, err = %v; want [] and no error", empty.String(), err)
	}
}

func TestHostsSortNumericallyWithSelf(t *testing.T) {
	hosts := []host{
		{IP: net.IPv4(192, 168, 12, 100).To4(), Via: []string{"icmp"}},
		{IP: net.IPv4(192, 168, 12, 2).To4(), Via: []string{"self"}},
		{IP: net.IPv4(192, 168, 12, 10).To4(), Via: []string{"icmp"}},
	}
	sortHosts(hosts)
	var out bytes.Buffer
	if err := writeTSV(&out, hosts); err != nil {
		t.Fatal(err)
	}
	want := "192.168.12.2\t0\tself\t\t\n192.168.12.10\t0\ticmp\t\t\n192.168.12.100\t0\ticmp\t\t\n"
	if out.String() != want {
		t.Errorf("sorted tsv = %q, want %q", out.String(), want)
	}
}

func TestSummaryLineFormat(t *testing.T) {
	var out bytes.Buffer
	writeSummary(&out, 3, "10.0.0.0/24", 1020*time.Millisecond)
	if got, want := out.String(), "3 hosts on 10.0.0.0/24 in 1020 ms\n"; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}
