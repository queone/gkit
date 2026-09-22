package main

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// neighborTable reads the system's IP-to-MAC table for goos. run executes a
// command and readFile reads a path; tests replace both. A goos without a
// known table returns an empty table and no error.
func neighborTable(goos string, run func(name string, args ...string) ([]byte, error), readFile func(name string) ([]byte, error)) (map[string]string, error) {
	switch goos {
	case "darwin", "freebsd", "netbsd", "openbsd":
		out, err := run("arp", "-an")
		if err != nil {
			return nil, fmt.Errorf("run arp -an: %w", err)
		}
		return parseARPTable(string(out)), nil
	case "linux":
		out, err := readFile("/proc/net/arp")
		if err != nil {
			return nil, fmt.Errorf("read /proc/net/arp: %w", err)
		}
		return parseProcNetARP(string(out)), nil
	}
	return map[string]string{}, nil
}

// arpEntry matches one complete `arp -an` line, such as
// `? (192.168.12.1) at 8c:3b:ad:d5:9:81 on en0 ifscope [ethernet]`.
var arpEntry = regexp.MustCompile(`\((\d+\.\d+\.\d+\.\d+)\) at ([0-9A-Fa-f:]+) `)

// parseARPTable reads `arp -an` output, skipping incomplete entries.
func parseARPTable(text string) map[string]string {
	out := map[string]string{}
	for line := range strings.SplitSeq(text, "\n") {
		m := arpEntry.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if mac, ok := normalizeMAC(m[2]); ok {
			out[m[1]] = mac
		}
	}
	return out
}

// parseProcNetARP reads /proc/net/arp, keeping entries whose flags mark them
// complete (bit 0x2).
func parseProcNetARP(text string) map[string]string {
	out := map[string]string{}
	sc := bufio.NewScanner(strings.NewReader(text))
	header := true
	for sc.Scan() {
		if header {
			header = false
			continue
		}
		f := strings.Fields(sc.Text())
		if len(f) < 4 {
			continue
		}
		flags, err := strconv.ParseInt(strings.TrimPrefix(f[2], "0x"), 16, 64)
		if err != nil || flags&0x2 == 0 {
			continue
		}
		if mac, ok := normalizeMAC(f[3]); ok {
			out[f[0]] = mac
		}
	}
	return out
}

// normalizeMAC lowercases a colon-separated MAC and pads each octet to two
// digits, since BSD arp prints "8c:3b:ad:1:2:3". An all-zero MAC is rejected.
func normalizeMAC(s string) (string, bool) {
	parts := strings.Split(strings.ToLower(s), ":")
	if len(parts) != 6 {
		return "", false
	}
	zero := true
	for i, p := range parts {
		if len(p) == 0 || len(p) > 2 {
			return "", false
		}
		if _, err := strconv.ParseUint(p, 16, 8); err != nil {
			return "", false
		}
		if len(p) == 1 {
			p = "0" + p
		}
		if p != "00" {
			zero = false
		}
		parts[i] = p
	}
	if zero {
		return "", false
	}
	return strings.Join(parts, ":"), true
}
