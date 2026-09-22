package main

import (
	"bytes"
	"context"
	"errors"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

var errTimeout = errors.New("i/o timeout")

// fakeICMP stands in for the ICMP socket. A reply registered with willReply is
// delivered only after the sweep has written an echo request to that address,
// as a real host would answer.
type fakeICMP struct {
	mu       sync.Mutex
	sent     []string
	pending  map[string]bool
	replies  chan net.Addr
	expired  chan struct{}
	once     sync.Once
	writeErr error
}

func newFakeICMP() *fakeICMP {
	return &fakeICMP{pending: map[string]bool{}, replies: make(chan net.Addr, 1024), expired: make(chan struct{})}
}

func (f *fakeICMP) willReply(ip string) { f.pending[ip] = true }

func (f *fakeICMP) WriteTo(b []byte, dst net.Addr) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ip := dst.(*net.UDPAddr).IP.String()
	f.sent = append(f.sent, ip)
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	if f.pending[ip] {
		f.replies <- &net.UDPAddr{IP: net.ParseIP(ip)}
	}
	return len(b), nil
}

func (f *fakeICMP) ReadFrom(b []byte) (int, net.Addr, error) {
	var peer net.Addr
	select {
	case peer = <-f.replies:
	default:
		select {
		case peer = <-f.replies:
		case <-f.expired:
			return 0, nil, errTimeout
		}
	}
	msg := icmp.Message{Type: ipv4.ICMPTypeEchoReply, Code: 0, Body: &icmp.Echo{ID: 1, Seq: 1, Data: []byte("x")}}
	wb, err := msg.Marshal(nil)
	if err != nil {
		return 0, nil, err
	}
	return copy(b, wb), peer, nil
}

func (f *fakeICMP) SetReadDeadline(t time.Time) error {
	time.AfterFunc(time.Until(t), func() { f.once.Do(func() { close(f.expired) }) })
	return nil
}

func (f *fakeICMP) Close() error {
	f.once.Do(func() { close(f.expired) })
	return nil
}

// testSweeper builds a sweeper for cidr whose ICMP socket is unavailable and
// whose dials all time out; tests override what they need.
func testSweeper(t *testing.T, cidr string, ports []int) (*sweeper, *bytes.Buffer) {
	t.Helper()
	n, err := parseCIDR(cidr)
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	s := newSweeper(n, nil, ports, 20*time.Millisecond, &stderr)
	s.listen = func() (icmpConn, error) { return nil, errors.New("icmp unavailable in tests") }
	s.dial = func(string, time.Duration) (net.Conn, error) { return nil, errTimeout }
	return s, &stderr
}

func viaByIP(hosts []host) map[string]string {
	out := map[string]string{}
	for _, h := range hosts {
		out[h.IP.String()] = strings.Join(h.Via, ",")
	}
	return out
}

func refused() error {
	return &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}
}

func TestSubnetHostsExcludeNetworkAndBroadcast(t *testing.T) {
	cases := []struct {
		cidr  string
		count int
		first string
		last  string
	}{
		{"192.168.12.0/24", 254, "192.168.12.1", "192.168.12.254"},
		{"10.1.2.4/30", 2, "10.1.2.5", "10.1.2.6"},
		{"10.0.0.0/16", 65534, "10.0.0.1", "10.0.255.254"},
	}
	for _, c := range cases {
		n, err := parseCIDR(c.cidr)
		if err != nil {
			t.Fatalf("%s: %v", c.cidr, err)
		}
		got := subnetHosts(n)
		if len(got) != c.count || got[0].String() != c.first || got[len(got)-1].String() != c.last {
			t.Errorf("%s: %d hosts %s..%s, want %d hosts %s..%s", c.cidr, len(got), got[0], got[len(got)-1], c.count, c.first, c.last)
		}
	}
}

func TestParseCIDRRejectsWideAndIPv6Subnets(t *testing.T) {
	if _, err := parseCIDR("10.0.0.0/15"); err == nil || !strings.Contains(err.Error(), "/16") {
		t.Errorf("/15: err = %v, want an error naming the /16 limit", err)
	}
	if _, err := parseCIDR("2001:db8::/64"); err == nil || !strings.Contains(err.Error(), "IPv6") {
		t.Errorf("IPv6: err = %v, want an IPv6 error", err)
	}
	if _, err := parseCIDR("nonsense"); err == nil {
		t.Error("nonsense parsed as a CIDR")
	}
	if n, err := parseCIDR("192.168.12.140/24"); err != nil || n.String() != "192.168.12.0/24" {
		t.Errorf("host CIDR: %v, %v; want the network 192.168.12.0/24", n, err)
	}
}

func TestEvidenceKeepsFirstRTTAndJoinsLabels(t *testing.T) {
	ev := newEvidence()
	ip := net.IPv4(10, 0, 0, 5).To4()
	ev.mark(ip, 7*time.Millisecond, "icmp")
	ev.mark(ip, 90*time.Millisecond, "tcp80")
	ev.mark(ip, time.Millisecond, "icmp")
	hosts := ev.list()
	if len(hosts) != 1 {
		t.Fatalf("hosts = %v, want one", hosts)
	}
	if hosts[0].RTT != 7*time.Millisecond || !reflect.DeepEqual(hosts[0].Via, []string{"icmp", "tcp80"}) {
		t.Errorf("host = %+v, want RTT 7ms and via [icmp tcp80]", hosts[0])
	}
}

func TestDialOutcomesBecomeEvidence(t *testing.T) {
	s, stderr := testSweeper(t, "10.9.9.0/29", []int{80})
	s.dial = func(addr string, _ time.Duration) (net.Conn, error) {
		switch addr {
		case "10.9.9.1:80":
			c, _ := net.Pipe()
			return c, nil
		case "10.9.9.2:80":
			return nil, refused()
		}
		return nil, errTimeout
	}
	rep, err := s.run()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := viaByIP(rep.hosts), map[string]string{"10.9.9.1": "tcp80", "10.9.9.2": "rst"}; !reflect.DeepEqual(got, want) {
		t.Errorf("hosts = %v, want %v", got, want)
	}
	if !strings.Contains(stderr.String(), "ping_group_range") {
		t.Errorf("stderr = %q, want the ICMP fallback warning naming the sysctl", stderr.String())
	}
}

func TestICMPRepliesMarkHosts(t *testing.T) {
	s, _ := testSweeper(t, "10.9.9.0/29", nil)
	f := newFakeICMP()
	f.willReply("10.9.9.3")
	s.listen = func() (icmpConn, error) { return f, nil }
	rep, err := s.run()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := viaByIP(rep.hosts), map[string]string{"10.9.9.3": "icmp"}; !reflect.DeepEqual(got, want) {
		t.Errorf("hosts = %v, want %v", got, want)
	}
	if len(f.sent) != 6 {
		t.Errorf("sent %d echo requests, want 6", len(f.sent))
	}
}

func TestFileDescriptorExhaustionIsCountedAndSweepContinues(t *testing.T) {
	s, _ := testSweeper(t, "10.9.9.0/29", []int{80})
	s.dial = func(addr string, _ time.Duration) (net.Conn, error) {
		if addr == "10.9.9.6:80" {
			c, _ := net.Pipe()
			return c, nil
		}
		return nil, &net.OpError{Op: "dial", Err: os.NewSyscallError("socket", syscall.EMFILE)}
	}
	rep, err := s.run()
	if err != nil {
		t.Fatal(err)
	}
	if rep.fdFails != 5 {
		t.Errorf("fdFails = %d, want 5", rep.fdFails)
	}
	if got, want := viaByIP(rep.hosts), map[string]string{"10.9.9.6": "tcp80"}; !reflect.DeepEqual(got, want) {
		t.Errorf("hosts = %v, want %v", got, want)
	}
}

func TestEveryProbeUnreachableIsReported(t *testing.T) {
	s, _ := testSweeper(t, "10.9.9.0/29", []int{80})
	f := newFakeICMP()
	f.writeErr = &net.OpError{Op: "write", Err: os.NewSyscallError("sendto", syscall.EHOSTUNREACH)}
	s.listen = func() (icmpConn, error) { return f, nil }
	s.dial = func(string, time.Duration) (net.Conn, error) {
		return nil, &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.EHOSTUNREACH)}
	}
	if _, err := s.run(); !errors.Is(err, errAllUnreachable) {
		t.Errorf("err = %v, want errAllUnreachable", err)
	}
}

func TestOneReachableProbeIsNotReportedAsUnreachable(t *testing.T) {
	s, _ := testSweeper(t, "10.9.9.0/29", []int{80})
	s.dial = func(addr string, _ time.Duration) (net.Conn, error) {
		if addr == "10.9.9.1:80" {
			return nil, refused()
		}
		return nil, &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.EHOSTUNREACH)}
	}
	rep, err := s.run()
	if err != nil {
		t.Fatal(err)
	}
	if got := viaByIP(rep.hosts); got["10.9.9.1"] != "rst" {
		t.Errorf("hosts = %v, want 10.9.9.1 via rst", got)
	}
}

func TestLocalHostIsListedAsSelfAndNotProbed(t *testing.T) {
	s, _ := testSweeper(t, "10.9.9.0/29", []int{80})
	s.local = net.IPv4(10, 9, 9, 4).To4()
	var mu sync.Mutex
	var dialed []string
	s.dial = func(addr string, _ time.Duration) (net.Conn, error) {
		mu.Lock()
		dialed = append(dialed, addr)
		mu.Unlock()
		return nil, errTimeout
	}
	rep, err := s.run()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range dialed {
		if strings.HasPrefix(a, "10.9.9.4:") {
			t.Errorf("dialed the local host: %s", a)
		}
	}
	if len(dialed) != 5 {
		t.Errorf("dialed %d addresses, want 5", len(dialed))
	}
	if got := viaByIP(rep.hosts); got["10.9.9.4"] != "self" {
		t.Errorf("hosts = %v, want 10.9.9.4 via self", got)
	}
}

func TestResolveNamesTrimsTheTrailingDot(t *testing.T) {
	hosts := []host{{IP: net.IPv4(10, 0, 0, 1).To4()}, {IP: net.IPv4(10, 0, 0, 2).To4()}}
	lookup := func(_ context.Context, addr string) ([]string, error) {
		if addr == "10.0.0.1" {
			return []string{"gw.lan."}, nil
		}
		return nil, errors.New("no name")
	}
	resolveNames(hosts, lookup)
	if hosts[0].Name != "gw.lan" || hosts[1].Name != "" {
		t.Errorf("names = %q, %q; want gw.lan and empty", hosts[0].Name, hosts[1].Name)
	}
}
