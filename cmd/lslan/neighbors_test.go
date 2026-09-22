package main

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestARPTableParsingSkipsIncompleteAndPadsOctets(t *testing.T) {
	text := "? (192.168.12.1) at 8c:3b:ad:d5:9:81 on en0 ifscope [ethernet]\n" +
		"? (192.168.12.55) at (incomplete) on en0 ifscope [ethernet]\n" +
		"? (192.168.12.234) at A4:CF:12:9:AB:CD on en0 ifscope [ethernet]\n"
	got := parseARPTable(text)
	want := map[string]string{"192.168.12.1": "8c:3b:ad:d5:09:81", "192.168.12.234": "a4:cf:12:09:ab:cd"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("table = %v, want %v", got, want)
	}
}

func TestProcNetARPKeepsCompleteEntries(t *testing.T) {
	text := "IP address       HW type     Flags       HW address            Mask     Device\n" +
		"192.168.0.50     0x1         0x2         00:50:BF:25:68:F3     *        eth0\n" +
		"192.168.0.250    0x1         0x0         00:00:00:00:00:00     *        eth0\n" +
		"192.168.0.9      0x1         0x6         00:50:bf:25:68:f4     *        eth0\n"
	got := parseProcNetARP(text)
	want := map[string]string{"192.168.0.50": "00:50:bf:25:68:f3", "192.168.0.9": "00:50:bf:25:68:f4"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("table = %v, want %v", got, want)
	}
}

func TestNeighborTablePicksTheReaderForTheOS(t *testing.T) {
	var ran, read string
	run := func(name string, args ...string) ([]byte, error) {
		ran = name + " " + strings.Join(args, " ")
		return []byte("? (10.0.0.1) at a:b:c:d:e:f on en0 ifscope [ethernet]\n"), nil
	}
	readFile := func(name string) ([]byte, error) {
		read = name
		return []byte("IP address HW type Flags HW address Mask Device\n10.0.0.2 0x1 0x2 00:11:22:33:44:55 * eth0\n"), nil
	}

	got, err := neighborTable("darwin", run, readFile)
	if err != nil || ran != "arp -an" || !reflect.DeepEqual(got, map[string]string{"10.0.0.1": "0a:0b:0c:0d:0e:0f"}) {
		t.Errorf("darwin: table = %v, err = %v, ran %q", got, err, ran)
	}
	got, err = neighborTable("linux", run, readFile)
	if err != nil || read != "/proc/net/arp" || !reflect.DeepEqual(got, map[string]string{"10.0.0.2": "00:11:22:33:44:55"}) {
		t.Errorf("linux: table = %v, err = %v, read %q", got, err, read)
	}
	ran, read = "", ""
	got, err = neighborTable("windows", run, readFile)
	if err != nil || len(got) != 0 || ran != "" || read != "" {
		t.Errorf("windows: table = %v, err = %v, ran %q, read %q; want an empty table and no reads", got, err, ran, read)
	}
	failing := func(string, ...string) ([]byte, error) { return nil, errors.New("exec: arp: not found") }
	if _, err := neighborTable("darwin", failing, readFile); err == nil || !strings.Contains(err.Error(), "arp -an") {
		t.Errorf("darwin failure: err = %v, want an error naming arp -an", err)
	}
}

func TestNormalizeMACRejectsMalformedAndZero(t *testing.T) {
	for _, bad := range []string{"", "0:0:0:0:0:0", "aa:bb:cc", "zz:11:22:33:44:55", "aaa:11:22:33:44:55"} {
		if mac, ok := normalizeMAC(bad); ok {
			t.Errorf("normalizeMAC(%q) = %q, want rejection", bad, mac)
		}
	}
}
