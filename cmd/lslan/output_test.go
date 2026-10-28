package main

import (
	"bytes"
	"net"
	"testing"
	"time"
)

func sampleHosts() []host {
	return []host{
		{IP: net.IPv4(10, 0, 0, 2).To4(), RTT: 3 * time.Millisecond, Via: []string{"icmp", "tcp80"}, Name: "gw.lan", MAC: "aa:bb:cc:dd:ee:ff", Vendor: "private"},
		{IP: net.IPv4(10, 0, 0, 9).To4(), Via: []string{"self"}},
	}
}

func TestTSVHasSixColumnsAndNoHeader(t *testing.T) {
	var out bytes.Buffer
	if err := writeTSV(&out, sampleHosts()); err != nil {
		t.Fatal(err)
	}
	want := "10.0.0.2\t3\ticmp,tcp80\tgw.lan\taa:bb:cc:dd:ee:ff\tprivate\n10.0.0.9\t0\tself\t\t\t\n"
	if out.String() != want {
		t.Errorf("tsv = %q, want %q", out.String(), want)
	}
	for line := range bytes.SplitSeq(bytes.TrimSuffix(out.Bytes(), []byte("\n")), []byte("\n")) {
		if n := bytes.Count(line, []byte("\t")); n != 5 {
			t.Errorf("line %q has %d tabs, want 5", line, n)
		}
	}
}

func TestJSONCarriesTheSixKeys(t *testing.T) {
	var out bytes.Buffer
	if err := writeJSON(&out, sampleHosts()); err != nil {
		t.Fatal(err)
	}
	want := `[{"ip":"10.0.0.2","rtt_ms":3,"via":["icmp","tcp80"],"name":"gw.lan","mac":"aa:bb:cc:dd:ee:ff","vendor":"private"},` +
		`{"ip":"10.0.0.9","rtt_ms":0,"via":["self"],"name":"","mac":"","vendor":""}]` + "\n"
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
	want := "192.168.12.2\t0\tself\t\t\t\n192.168.12.10\t0\ticmp\t\t\t\n192.168.12.100\t0\ticmp\t\t\t\n"
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

func TestTableAlignsColumnsUnderAHeader(t *testing.T) {
	hosts := append(sampleHosts(), host{IP: net.IPv4(10, 0, 0, 120).To4(), RTT: 1234 * time.Millisecond, Via: []string{"tcp443"}, Name: "a-much-longer-name.lan"})
	var out bytes.Buffer
	if err := writeTable(&out, hosts); err != nil {
		t.Fatal(err)
	}
	want := "IP          RTT   VIA         NAME                    MAC                VENDOR\n" +
		"10.0.0.2    3     icmp,tcp80  gw.lan                  aa:bb:cc:dd:ee:ff  private\n" +
		"10.0.0.9    0     self\n" +
		"10.0.0.120  1234  tcp443      a-much-longer-name.lan\n"
	if out.String() != want {
		t.Errorf("table =\n%s\nwant\n%s", out.String(), want)
	}
}
