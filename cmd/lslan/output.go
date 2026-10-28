package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"
)

// jsonHost is the --json shape of one host.
type jsonHost struct {
	IP     string   `json:"ip"`
	RTTMs  int64    `json:"rtt_ms"`
	Via    []string `json:"via"`
	Name   string   `json:"name"`
	MAC    string   `json:"mac"`
	Vendor string   `json:"vendor"`
}

// writeTSV prints one tab-separated line per host, with no header: IP, RTT in
// whole milliseconds, comma-joined evidence, name, MAC, vendor.
func writeTSV(w io.Writer, hosts []host) error {
	for _, h := range hosts {
		if _, err := fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\t%s\n", h.IP, h.RTT.Milliseconds(), strings.Join(h.Via, ","), h.Name, h.MAC, h.Vendor); err != nil {
			return err
		}
	}
	return nil
}

// writeTable prints the hosts for a terminal: a header row, then one row per
// host, with the columns aligned by spaces and no trailing spaces.
func writeTable(w io.Writer, hosts []host) error {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	fmt.Fprint(tw, "IP\tRTT\tVIA\tNAME\tMAC\tVENDOR\n")
	for _, h := range hosts {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t%s\n", h.IP, h.RTT.Milliseconds(), strings.Join(h.Via, ","), h.Name, h.MAC, h.Vendor)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	for line := range strings.SplitSeq(strings.TrimSuffix(buf.String(), "\n"), "\n") {
		if _, err := fmt.Fprintln(w, strings.TrimRight(line, " ")); err != nil {
			return err
		}
	}
	return nil
}

// writeJSON prints the hosts as one JSON array followed by a newline.
func writeJSON(w io.Writer, hosts []host) error {
	out := make([]jsonHost, 0, len(hosts))
	for _, h := range hosts {
		via := h.Via
		if via == nil {
			via = []string{}
		}
		out = append(out, jsonHost{IP: h.IP.String(), RTTMs: h.RTT.Milliseconds(), Via: via, Name: h.Name, MAC: h.MAC, Vendor: h.Vendor})
	}
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", b)
	return err
}

// writeSummary prints the one-line stderr summary.
func writeSummary(w io.Writer, n int, subnet string, elapsed time.Duration) {
	fmt.Fprintf(w, "%d hosts on %s in %d ms\n", n, subnet, elapsed.Milliseconds())
}
