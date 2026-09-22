package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// jsonHost is the --json shape of one host.
type jsonHost struct {
	IP    string   `json:"ip"`
	RTTMs int64    `json:"rtt_ms"`
	Via   []string `json:"via"`
	Name  string   `json:"name"`
	MAC   string   `json:"mac"`
}

// writeTSV prints one tab-separated line per host, with no header: IP, RTT in
// whole milliseconds, comma-joined evidence, name, MAC.
func writeTSV(w io.Writer, hosts []host) error {
	for _, h := range hosts {
		if _, err := fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\n", h.IP, h.RTT.Milliseconds(), strings.Join(h.Via, ","), h.Name, h.MAC); err != nil {
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
		out = append(out, jsonHost{IP: h.IP.String(), RTTMs: h.RTT.Milliseconds(), Via: via, Name: h.Name, MAC: h.MAC})
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
