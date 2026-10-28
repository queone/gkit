package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
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
	values := map[string]string{"-w": "250", "-p": "22,443", "-s": "10.0.0.1"}
	for _, pair := range [][]string{{"-n", "--numeric"}, {"-j", "--json"}, {"-w", "--wait"}, {"-p", "--ports"}, {"-s", "--server"}} {
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
	for _, args := range [][]string{{"-w"}, {"--wait", "soon"}, {"-p"}, {"--ports", "http"}, {"-p", "0"}, {"-s"}, {"--server", "router.local"}, {"-s", "300.1.1.1"}, {"-s", "10.0.0.1:x"}} {
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
	if got, want := stdout.String(), "10.0.0.7\t0\ticmp\t\t\t\n"; got != want {
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
	if got, want := stdout.String(), "10.0.0.7\t0\ticmp\tbox.lan\taa:bb:cc:dd:ee:ff\tprivate\n"; got != want {
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
	d.mdns = func(net.IP, []net.IP) map[string]mdnsInfo {
		t.Error("mDNS ran with -n")
		return nil
	}
	if rc := run([]string{"-n", "10.0.0.0/24"}, d); rc != 0 {
		t.Errorf("exit = %d, want 0", rc)
	}
	if got, want := stdout.String(), "10.0.0.7\t0\ticmp\t\t\t\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestJSONFlagPrintsAnArray(t *testing.T) {
	stdout, _, d := testDeps()
	d.sweep = oneHostSweep
	if rc := run([]string{"-j", "-n", "10.0.0.0/24"}, d); rc != 0 {
		t.Errorf("exit = %d, want 0", rc)
	}
	if got, want := stdout.String(), `[{"ip":"10.0.0.7","rtt_ms":0,"via":["icmp"],"name":"","mac":"","vendor":""}]`+"\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func oneHostSweep(*sweeper) (sweepReport, error) {
	return sweepReport{hosts: []host{{IP: net.IPv4(10, 0, 0, 7).To4(), Via: []string{"icmp"}}}}, nil
}

func noName(context.Context, string) ([]string, error) { return nil, errors.New("no name") }

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
		dnsAt:         func(string) func(context.Context, string) ([]string, error) { return noName },
		gateways:      func() ([]net.IP, error) { return nil, errors.New("no route table") },
		mdns:          func(net.IP, []net.IP) map[string]mdnsInfo { return nil },
		neighbors:     func() (map[string]string, error) { return map[string]string{}, nil },
		terminal:      func() bool { return false },
	}
	return &stdout, &stderr, d
}

func TestServerAddrFormsAndConflict(t *testing.T) {
	for in, want := range map[string]string{"10.0.0.1": "10.0.0.1:53", "10.0.0.1:5353": "10.0.0.1:5353"} {
		o, err := parseArgs([]string{"-s", in})
		if err != nil || o.server != want {
			t.Errorf("-s %s: server %q err %v, want %q", in, o.server, err, want)
		}
	}
	_, err := parseArgs([]string{"-s", "router.local"})
	if err == nil || !strings.Contains(err.Error(), "IPv4 address with an optional :PORT") {
		t.Errorf("-s router.local: err %v, want the expected form", err)
	}
	if _, err := parseArgs([]string{"-s", "10.0.0.1", "-n"}); err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Errorf("-s with -n: err %v, want a conflict error", err)
	}
}

// ptrServer answers every PTR query on a local UDP port with name.
func ptrServer(t *testing.T, name string) string {
	t.Helper()
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })
	go func() {
		buf := make([]byte, 512)
		for {
			n, from, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			var p dnsmessage.Parser
			h, err := p.Start(buf[:n])
			if err != nil {
				continue
			}
			q, err := p.Question()
			if err != nil {
				continue
			}
			b := dnsmessage.NewBuilder(nil, dnsmessage.Header{ID: h.ID, Response: true, Authoritative: true})
			b.EnableCompression()
			_ = b.StartQuestions()
			_ = b.Question(q)
			_ = b.StartAnswers()
			_ = b.PTRResource(dnsmessage.ResourceHeader{Name: q.Name, Type: dnsmessage.TypePTR, Class: dnsmessage.ClassINET, TTL: 60},
				dnsmessage.PTRResource{PTR: dnsmessage.MustNewName(name)})
			msg, err := b.Finish()
			if err == nil {
				pc.WriteTo(msg, from)
			}
		}
	}()
	return pc.LocalAddr().String()
}

func TestServerFlagAsksOnlyThatServer(t *testing.T) {
	addr := ptrServer(t, "router.lan.")
	names, err := dnsAt(addr)(context.Background(), "10.0.0.7")
	if err != nil || len(names) == 0 || names[0] != "router.lan." {
		t.Fatalf("dnsAt(%s) = %v %v, want router.lan.", addr, names, err)
	}

	stdout, _, d := testDeps()
	d.sweep = oneHostSweep
	d.dnsAt = dnsAt
	d.lookup = func(context.Context, string) ([]string, error) {
		t.Error("system lookup called with -s")
		return nil, nil
	}
	d.gateways = func() ([]net.IP, error) {
		t.Error("gateway discovery ran with -s")
		return nil, nil
	}
	d.mdns = func(net.IP, []net.IP) map[string]mdnsInfo {
		t.Error("mDNS ran with -s")
		return nil
	}
	if rc := run([]string{"-s", addr, "10.0.0.0/24"}, d); rc != 0 {
		t.Fatalf("exit = %d, want 0", rc)
	}
	if got, want := stdout.String(), "10.0.0.7\t0\ticmp\trouter.lan\t\t\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestGatewayNamesWhatTheSystemResolverCannot(t *testing.T) {
	twoHosts := func(*sweeper) (sweepReport, error) {
		return sweepReport{hosts: []host{
			{IP: net.IPv4(10, 0, 0, 1).To4(), Via: []string{"icmp"}},
			{IP: net.IPv4(10, 0, 0, 7).To4(), Via: []string{"icmp"}},
		}}, nil
	}
	stdout, _, d := testDeps()
	d.sweep = twoHosts
	d.lookup = func(_ context.Context, addr string) ([]string, error) {
		if addr == "10.0.0.7" {
			return []string{"box.lan."}, nil
		}
		return nil, errors.New("no name")
	}
	var asked []string
	d.gateways = func() ([]net.IP, error) { return []net.IP{net.IPv4(100, 64, 0, 1), net.IPv4(10, 0, 0, 1)}, nil }
	d.dnsAt = func(server string) func(context.Context, string) ([]string, error) {
		return func(_ context.Context, addr string) ([]string, error) {
			asked = append(asked, server+" "+addr)
			return []string{"router.lan."}, nil
		}
	}
	if rc := run([]string{"10.0.0.0/24"}, d); rc != 0 {
		t.Fatalf("exit = %d, want 0", rc)
	}
	if want := []string{"10.0.0.1:53 10.0.0.1"}; !reflect.DeepEqual(asked, want) {
		t.Errorf("gateway asked %v, want %v", asked, want)
	}
	if got, want := stdout.String(), "10.0.0.1\t0\ticmp\trouter.lan\t\t\n10.0.0.7\t0\ticmp\tbox.lan\t\t\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	stdout, stderr, d := testDeps()
	d.sweep = twoHosts
	d.dnsAt = func(string) func(context.Context, string) ([]string, error) {
		t.Error("gateway lookup ran without a gateway")
		return nil
	}
	if rc := run([]string{"10.0.0.0/24"}, d); rc != 0 {
		t.Fatalf("exit = %d, want 0", rc)
	}
	if got, want := stdout.String(), "10.0.0.1\t0\ticmp\t\t\t\n10.0.0.7\t0\ticmp\t\t\t\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if !strings.HasPrefix(stderr.String(), "2 hosts on") {
		t.Errorf("stderr = %q, want only the summary", stderr.String())
	}
}

func TestDefaultGatewaysPerOS(t *testing.T) {
	darwin := "Routing tables\n\nInternet:\nDestination        Gateway            Flags               Netif Expire\n" +
		"default            192.168.12.1       UGScg                 en1\ndefault            link#34            UCSIg              utun10\n" +
		"127                127.0.0.1          UCS                     lo0\n"
	linux := "Iface\tDestination\tGateway \tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n" +
		"eth0\t00000000\t0101A8C0\t0003\t0\t0\t100\t00000000\t0\t0\t0\n" +
		"eth0\t0001A8C0\t00000000\t0001\t0\t0\t100\t00FFFFFF\t0\t0\t0\n"
	windows := "IPv4 Route Table\n===\nActive Routes:\nNetwork Destination        Netmask          Gateway       Interface  Metric\n" +
		"          0.0.0.0          0.0.0.0      192.168.0.1    192.168.0.20     25\n===\nPersistent Routes:\n  None\n"
	for _, tc := range []struct {
		goos, out, want string
	}{
		{"darwin", darwin, "192.168.12.1"},
		{"linux", linux, "192.168.1.1"},
		{"windows", windows, "192.168.0.1"},
	} {
		run := func(string, ...string) ([]byte, error) { return []byte(tc.out), nil }
		read := func(string) ([]byte, error) { return []byte(tc.out), nil }
		gws, err := defaultGateways(tc.goos, run, read)
		if err != nil || len(gws) != 1 || gws[0].String() != tc.want {
			t.Errorf("%s: gateways %v err %v, want [%s]", tc.goos, gws, err, tc.want)
		}
		none := func(string, ...string) ([]byte, error) { return []byte("Destination Gateway\n"), nil }
		noneFile := func(string) ([]byte, error) { return []byte("Iface\tDestination\tGateway\n"), nil }
		if _, err := defaultGateways(tc.goos, none, noneFile); !errors.Is(err, errNoDefaultRoute) {
			t.Errorf("%s with no default route: err %v, want errNoDefaultRoute", tc.goos, err)
		}
	}
	if _, err := defaultGateways("plan9", nil, nil); err == nil {
		t.Error("plan9: want an error")
	}
	_, subnet, _ := net.ParseCIDR("192.168.12.0/24")
	if gw := gatewayIn(subnet, []net.IP{net.IPv4(100, 64, 0, 1), net.IPv4(192, 168, 12, 1)}); fmt.Sprint(gw) != "192.168.12.1" {
		t.Errorf("gatewayIn = %v, want 192.168.12.1", gw)
	}
}

func TestTerminalGetsTheTableAndPipesGetTSV(t *testing.T) {
	stdout, _, d := testDeps()
	d.sweep = oneHostSweep
	d.terminal = func() bool { return true }
	if rc := run([]string{"-n", "10.0.0.0/24"}, d); rc != 0 {
		t.Fatalf("exit = %d, want 0", rc)
	}
	if got, want := stdout.String(), "IP        RTT  VIA   NAME  MAC  VENDOR\n10.0.0.7  0    icmp\n"; got != want {
		t.Errorf("terminal stdout = %q, want %q", got, want)
	}

	stdout, _, d = testDeps()
	d.sweep = oneHostSweep
	d.terminal = func() bool { return true }
	if rc := run([]string{"-j", "-n", "10.0.0.0/24"}, d); rc != 0 || !strings.HasPrefix(stdout.String(), "[{") {
		t.Errorf("terminal -j: exit %d stdout %q, want JSON", rc, stdout.String())
	}
}

func TestMDNSNameReplacesDNSNameAndRefinesVendor(t *testing.T) {
	stdout, _, d := testDeps()
	d.sweep = func(*sweeper) (sweepReport, error) {
		return sweepReport{hosts: []host{
			{IP: net.IPv4(10, 0, 0, 1).To4(), Via: []string{"icmp"}},
			{IP: net.IPv4(10, 0, 0, 7).To4(), Via: []string{"icmp"}},
		}}, nil
	}
	d.lookup = func(_ context.Context, addr string) ([]string, error) {
		return map[string][]string{"10.0.0.1": {"router.lan."}, "10.0.0.7": {"iPhone.lan."}}[addr], nil
	}
	var asked []string
	d.mdns = func(_ net.IP, ips []net.IP) map[string]mdnsInfo {
		for _, ip := range ips {
			asked = append(asked, ip.String())
		}
		return map[string]mdnsInfo{"10.0.0.7": {Name: "np17.local"}}
	}
	d.neighbors = func() (map[string]string, error) {
		return map[string]string{"10.0.0.1": "18:a5:ff:9d:6e:24", "10.0.0.7": "b2:b0:5c:16:8e:6d"}, nil
	}
	if rc := run([]string{"10.0.0.0/24"}, d); rc != 0 {
		t.Fatalf("exit = %d, want 0", rc)
	}
	if want := []string{"10.0.0.1", "10.0.0.7"}; !reflect.DeepEqual(asked, want) {
		t.Errorf("mDNS asked %v, want every host %v", asked, want)
	}
	want := "10.0.0.1\t0\ticmp\trouter.lan\t18:a5:ff:9d:6e:24\tArcadyan Corporation\n" +
		"10.0.0.7\t0\ticmp\tnp17.local\tb2:b0:5c:16:8e:6d\tprivate (Apple)\n"
	if got := stdout.String(); got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}
