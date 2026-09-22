// lslan lists the live hosts on the local network by probing every address in
// the subnet from an unprivileged socket. See README.md in this directory.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/queone/gkit/internal/help"
)

const (
	programName    = "lslan"
	programVersion = "1.0.0"
)

// defaultWait is how long the sweep listens for late ICMP replies.
const defaultWait = 500 * time.Millisecond

// defaultPorts are the TCP ports probed when -p is not given.
const defaultPorts = "22,80,443"

// options is the parsed command line.
type options struct {
	help    bool
	version bool
	numeric bool
	json    bool
	wait    time.Duration
	ports   []int
	cidr    string
}

func helpDoc() help.Doc {
	return help.Doc{
		Name:        programName,
		Version:     programVersion,
		Description: "List the live hosts on the local network",
		URL:         help.URL(programName),
		Sections: []help.Section{
			{Title: "Usage", Rows: []help.Row{
				{Form: "lslan [OPTIONS] [CIDR]", Meaning: "List the hosts that answer on the local subnet, or on CIDR"},
			}, Lines: []string{
				"Without CIDR, lslan sweeps the IPv4 subnet of the interface that routes to",
				"the internet. A prefix wider than /16 is rejected; pass a narrower CIDR.",
			}},
			{Title: "Options", Rows: []help.Row{
				{Form: "-n, --numeric", Meaning: "Skip reverse-DNS lookups and leave NAME empty"},
				{Form: "-j, --json", Meaning: "Print a JSON array instead of tab-separated lines"},
				{Form: "-w, --wait MS", Meaning: "Wait MS milliseconds for late ICMP replies; default 500"},
				{Form: "-p, --ports LIST", Meaning: "Probe these TCP ports; default 22,80,443, and an empty LIST disables them"},
			}},
			{Title: "Columns", Rows: []help.Row{
				{Form: "IP", Meaning: "IPv4 address, in numeric order"},
				{Form: "RTT", Meaning: "Milliseconds until the first proof that the host is up"},
				{Form: "VIA", Meaning: "Evidence: icmp, tcpNN for an open port, rst for a closed one, self"},
				{Form: "NAME", Meaning: "Reverse-DNS name, or empty"},
				{Form: "MAC", Meaning: "From the neighbor table when readable; macOS 27 needs sudo"},
			}},
			{Title: "Examples", Rows: []help.Row{
				{Form: "lslan", Meaning: "List the local subnet"},
				{Form: "lslan -n 10.0.5.0/24", Meaning: "List another subnet without name lookups"},
				{Form: "sudo lslan -j | jq .", Meaning: "JSON with MAC addresses on macOS"},
			}},
		},
	}
}

// parseArgs reads the command line. Flags may appear in any order around the
// optional CIDR.
func parseArgs(args []string) (options, error) {
	o := options{wait: defaultWait}
	o.ports, _ = parsePorts(defaultPorts)
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-h", "-?", "--help":
			o.help = true
		case "-v", "--version":
			o.version = true
		case "-n", "--numeric":
			o.numeric = true
		case "-j", "--json":
			o.json = true
		case "-w", "--wait":
			if i+1 >= len(args) {
				return o, fmt.Errorf("%s requires MS, the milliseconds to wait for ICMP replies", a)
			}
			ms, err := strconv.Atoi(args[i+1])
			if err != nil || ms < 0 {
				return o, fmt.Errorf("%s requires a whole number of milliseconds, got %q", a, args[i+1])
			}
			o.wait = time.Duration(ms) * time.Millisecond
			i++
		case "-p", "--ports":
			if i+1 >= len(args) {
				return o, fmt.Errorf("%s requires LIST, comma-separated TCP ports", a)
			}
			ports, err := parsePorts(args[i+1])
			if err != nil {
				return o, fmt.Errorf("%s %w", a, err)
			}
			o.ports = ports
			i++
		default:
			if strings.HasPrefix(a, "-") {
				return o, fmt.Errorf("unknown option %s", a)
			}
			if o.cidr != "" {
				return o, fmt.Errorf("expected at most one CIDR, got %q and %q", o.cidr, a)
			}
			o.cidr = a
		}
	}
	return o, nil
}

// parsePorts reads a comma-separated port list; an empty list disables TCP probes.
func parsePorts(s string) ([]int, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	var out []int
	for f := range strings.SplitSeq(s, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("requires a comma-separated list of ports 1-65535, got %q", s)
		}
		out = append(out, n)
	}
	return out, nil
}

// deps are the process-level dependencies run needs; tests replace them.
type deps struct {
	stdout        io.Writer
	stderr        io.Writer
	goos          string
	euid          func() int
	defaultSubnet func() (*net.IPNet, net.IP, error)
	sweep         func(s *sweeper) (sweepReport, error)
	lookup        func(ctx context.Context, addr string) ([]string, error)
	neighbors     func() (map[string]string, error)
}

func newDeps() deps {
	return deps{
		stdout:        os.Stdout,
		stderr:        os.Stderr,
		goos:          runtime.GOOS,
		euid:          os.Geteuid,
		defaultSubnet: defaultSubnet,
		sweep:         (*sweeper).run,
		lookup:        net.DefaultResolver.LookupAddr,
		neighbors:     systemNeighbors,
	}
}

// systemNeighbors reads this system's neighbor table.
func systemNeighbors() (map[string]string, error) {
	run := func(name string, args ...string) ([]byte, error) { return exec.Command(name, args...).Output() }
	return neighborTable(runtime.GOOS, run, os.ReadFile)
}

// run executes one listing and returns the exit status: 0 when the sweep ran,
// 1 for a bad command line, 2 when the sweep cannot run.
func run(args []string, d deps) int {
	o, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(d.stderr, "%s: %v\n\n%s", programName, err, helpDoc().String())
		return 1
	}
	if o.help {
		fmt.Fprint(d.stdout, helpDoc().String())
		return 0
	}
	if o.version {
		fmt.Fprintf(d.stdout, "%s v%s\n", programName, programVersion)
		return 0
	}

	start := time.Now()
	var subnet *net.IPNet
	var local net.IP
	if o.cidr != "" {
		subnet, err = parseCIDR(o.cidr)
		if _, ip, derr := d.defaultSubnet(); derr == nil {
			local = ip
		}
	} else {
		subnet, local, err = d.defaultSubnet()
		if err != nil {
			err = fmt.Errorf("%s: no IPv4 interface routes to the internet (%v); pass a CIDR such as 192.168.1.0/24", programName, err)
		} else {
			subnet, err = checkSubnet(subnet)
		}
	}
	if err != nil {
		fmt.Fprintln(d.stderr, err)
		return 2
	}

	rep, err := d.sweep(newSweeper(subnet, local, o.ports, o.wait, d.stderr))
	if err != nil {
		if errors.Is(err, errAllUnreachable) {
			fmt.Fprintln(d.stderr, unreachableMessage(d.goos))
			return 2
		}
		fmt.Fprintf(d.stderr, "%s: sweep failed: %v\n", programName, err)
		return 2
	}
	if rep.fdFails > 0 {
		fmt.Fprintf(d.stderr, "%s: %d probes failed because the process ran out of file descriptors, so hosts may be missing; raise the open-file limit with ulimit -n and rerun\n", programName, rep.fdFails)
	}
	hosts := rep.hosts
	if !o.numeric {
		resolveNames(hosts, d.lookup)
	}
	table, err := d.neighbors()
	if err != nil && d.euid() == 0 {
		fmt.Fprintf(d.stderr, "%s: cannot read the neighbor table (%v); listing without MAC addresses\n", programName, err)
	}
	for i := range hosts {
		hosts[i].MAC = table[hosts[i].IP.String()]
	}
	sortHosts(hosts)
	if o.json {
		err = writeJSON(d.stdout, hosts)
	} else {
		err = writeTSV(d.stdout, hosts)
	}
	if err != nil {
		fmt.Fprintf(d.stderr, "%s: write output: %v\n", programName, err)
		return 2
	}
	writeSummary(d.stderr, len(hosts), subnet.String(), time.Since(start))
	return 0
}

// unreachableMessage explains a sweep in which every probe had no route.
func unreachableMessage(goos string) string {
	if goos == "darwin" {
		return programName + ": every probe failed with no route to host; the terminal application lacks Local Network permission. Allow it under System Settings > Privacy & Security > Local Network and rerun"
	}
	return programName + ": every probe failed with no route to host; check that the subnet is reachable from this interface"
}

func main() {
	os.Exit(run(os.Args[1:], newDeps()))
}
