package main

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestVersionAliases(t *testing.T) {
	const envName = "GKIT_LSLAN_VERSION_FLAG"
	if flag := os.Getenv(envName); flag != "" {
		os.Args = []string{programName, flag}
		main()
		os.Exit(0)
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("locate test binary: %v", err)
	}
	for _, flag := range []string{"-v", "--version"} {
		t.Run(flag, func(t *testing.T) {
			cmd := exec.Command(exe, "-test.run=^TestVersionAliases$")
			cmd.Env = append(os.Environ(), envName+"="+flag)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("run %s: %v", flag, err)
			}
			if got, want := stdout.String(), programName+" v"+programVersion+"\n"; got != want {
				t.Errorf("stdout = %q, want %q", got, want)
			}
			if got := stderr.String(); got != "" {
				t.Errorf("stderr = %q, want empty", got)
			}
		})
	}
}

// The help screen follows the shared standard: three header lines, then the
// sections the renderer accepts, with the standard rows left to the renderer.
func TestHelpDocFollowsTheStandard(t *testing.T) {
	if err := helpDoc().Check(); err != nil {
		t.Fatal(err)
	}
}

func TestShortAndLongFlagsParseAlike(t *testing.T) {
	values := map[string]string{"-w": "250", "-p": "22,443"}
	for _, pair := range [][]string{{"-n", "--numeric"}, {"-j", "--json"}, {"-w", "--wait"}, {"-p", "--ports"}} {
		short, long := []string{pair[0]}, []string{pair[1]}
		if v, ok := values[pair[0]]; ok {
			short = append(short, v)
			long = append(long, v)
		}
		a, err := parseArgs(short)
		if err != nil {
			t.Fatalf("parse %q: %v", short, err)
		}
		b, err := parseArgs(long)
		if err != nil {
			t.Fatalf("parse %q: %v", long, err)
		}
		if !reflect.DeepEqual(a, b) {
			t.Errorf("%q parsed as %+v but %q as %+v", short, a, long, b)
		}
	}
	o, err := parseArgs([]string{"-w", "250", "-p", "22,443", "10.0.0.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	if o.wait != 250*time.Millisecond || !reflect.DeepEqual(o.ports, []int{22, 443}) || o.cidr != "10.0.0.0/24" {
		t.Errorf("parsed %+v, want wait 250ms, ports [22 443], cidr 10.0.0.0/24", o)
	}
}

func TestBadFlagValuesNameTheFlag(t *testing.T) {
	for _, args := range [][]string{{"-w"}, {"--wait", "soon"}, {"-p"}, {"--ports", "http"}, {"-p", "0"}} {
		_, err := parseArgs(args)
		if err == nil || !strings.Contains(err.Error(), args[0]) {
			t.Errorf("parse %q: err = %v, want an error naming %s", args, err, args[0])
		}
	}
}

func TestEmptyPortListDisablesTCPProbes(t *testing.T) {
	o, err := parseArgs([]string{"-p", ""})
	if err != nil {
		t.Fatal(err)
	}
	if len(o.ports) != 0 {
		t.Errorf("ports = %v, want none", o.ports)
	}
}

func TestSecondCIDRIsAUsageError(t *testing.T) {
	if _, err := parseArgs([]string{"10.0.0.0/24", "10.0.1.0/24"}); err == nil {
		t.Error("two CIDRs parsed without error")
	}
	stdout, stderr, d := testDeps()
	if rc := run([]string{"10.0.0.0/24", "10.0.1.0/24"}, d); rc != 1 {
		t.Errorf("exit = %d, want 1", rc)
	}
	if !strings.Contains(stderr.String(), "Usage") || stdout.Len() != 0 {
		t.Errorf("stderr = %q, stdout = %q; want the help on stderr only", stderr.String(), stdout.String())
	}
}

func TestUnknownOptionIsAUsageError(t *testing.T) {
	_, _, d := testDeps()
	if rc := run([]string{"--bogus"}, d); rc != 1 {
		t.Errorf("exit = %d, want 1", rc)
	}
}

func TestNoDefaultSubnetSuggestsACIDR(t *testing.T) {
	_, stderr, d := testDeps()
	if rc := run(nil, d); rc != 2 {
		t.Errorf("exit = %d, want 2", rc)
	}
	if !strings.Contains(stderr.String(), "pass a CIDR") {
		t.Errorf("stderr = %q, want the CIDR suggestion", stderr.String())
	}
}

func TestUnreachableProbesReportLocalNetworkPermission(t *testing.T) {
	_, stderr, d := testDeps()
	d.goos = "darwin"
	d.sweep = func(*sweeper) (sweepReport, error) { return sweepReport{}, errAllUnreachable }
	if rc := run([]string{"10.0.0.0/24"}, d); rc != 2 {
		t.Errorf("exit = %d, want 2", rc)
	}
	if !strings.Contains(stderr.String(), "Local Network") {
		t.Errorf("stderr = %q, want the Local Network permission message", stderr.String())
	}
}

func TestFileDescriptorWarningStillLists(t *testing.T) {
	stdout, stderr, d := testDeps()
	d.sweep = func(*sweeper) (sweepReport, error) {
		return sweepReport{hosts: []host{{IP: net.IPv4(10, 0, 0, 7).To4(), Via: []string{"icmp"}}}, fdFails: 3}, nil
	}
	if rc := run([]string{"10.0.0.0/24"}, d); rc != 0 {
		t.Errorf("exit = %d, want 0", rc)
	}
	if !strings.Contains(stderr.String(), "open-file limit") {
		t.Errorf("stderr = %q, want the open-file warning", stderr.String())
	}
	if got, want := stdout.String(), "10.0.0.7\t0\ticmp\t\t\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestNeighborFailureWarnsOnlyAsRoot(t *testing.T) {
	for _, euid := range []int{0, 501} {
		_, stderr, d := testDeps()
		d.euid = func() int { return euid }
		d.neighbors = func() (map[string]string, error) { return nil, errors.New("arp: not found") }
		if rc := run([]string{"10.0.0.0/24"}, d); rc != 0 {
			t.Errorf("euid %d: exit = %d, want 0", euid, rc)
		}
		if warned := strings.Contains(stderr.String(), "neighbor table"); warned != (euid == 0) {
			t.Errorf("euid %d: stderr = %q; warned = %v", euid, stderr.String(), warned)
		}
	}
}

func TestMACsAttachAndSummaryPrints(t *testing.T) {
	stdout, stderr, d := testDeps()
	d.sweep = oneHostSweep
	d.neighbors = func() (map[string]string, error) { return map[string]string{"10.0.0.7": "aa:bb:cc:dd:ee:ff"}, nil }
	d.lookup = func(context.Context, string) ([]string, error) { return []string{"box.lan."}, nil }
	if rc := run([]string{"10.0.0.0/24"}, d); rc != 0 {
		t.Errorf("exit = %d, want 0", rc)
	}
	if got, want := stdout.String(), "10.0.0.7\t0\ticmp\tbox.lan\taa:bb:cc:dd:ee:ff\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if !strings.HasPrefix(stderr.String(), "1 hosts on 10.0.0.0/24 in ") {
		t.Errorf("stderr = %q, want the summary line", stderr.String())
	}
}

func TestNumericSkipsLookups(t *testing.T) {
	stdout, _, d := testDeps()
	d.sweep = oneHostSweep
	d.lookup = func(context.Context, string) ([]string, error) {
		t.Error("lookup called with -n")
		return nil, nil
	}
	if rc := run([]string{"-n", "10.0.0.0/24"}, d); rc != 0 {
		t.Errorf("exit = %d, want 0", rc)
	}
	if got, want := stdout.String(), "10.0.0.7\t0\ticmp\t\t\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestJSONFlagPrintsAnArray(t *testing.T) {
	stdout, _, d := testDeps()
	d.sweep = oneHostSweep
	if rc := run([]string{"-j", "-n", "10.0.0.0/24"}, d); rc != 0 {
		t.Errorf("exit = %d, want 0", rc)
	}
	if got, want := stdout.String(), `[{"ip":"10.0.0.7","rtt_ms":0,"via":["icmp"],"name":"","mac":""}]`+"\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func oneHostSweep(*sweeper) (sweepReport, error) {
	return sweepReport{hosts: []host{{IP: net.IPv4(10, 0, 0, 7).To4(), Via: []string{"icmp"}}}}, nil
}

// testDeps returns buffers and dependencies that touch no network: no default
// subnet, an empty sweep, failing lookups, and an empty neighbor table.
func testDeps() (*bytes.Buffer, *bytes.Buffer, deps) {
	var stdout, stderr bytes.Buffer
	d := deps{
		stdout:        &stdout,
		stderr:        &stderr,
		goos:          "linux",
		euid:          func() int { return 501 },
		defaultSubnet: func() (*net.IPNet, net.IP, error) { return nil, nil, errors.New("no route") },
		sweep:         func(*sweeper) (sweepReport, error) { return sweepReport{}, nil },
		lookup:        func(context.Context, string) ([]string, error) { return nil, errors.New("no name") },
		neighbors:     func() (map[string]string, error) { return map[string]string{}, nil },
	}
	return &stdout, &stderr, d
}
