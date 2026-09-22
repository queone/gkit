package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// minPrefix is the widest subnet lslan sweeps; wider prefixes are rejected.
const minPrefix = 16

// maxInFlight caps the concurrent TCP probes.
const maxInFlight = 1024

// tcpTimeout bounds each TCP connect probe.
const tcpTimeout = 600 * time.Millisecond

// dnsTimeout bounds each reverse-DNS lookup.
const dnsTimeout = 500 * time.Millisecond

// icmpProtocol is the IANA protocol number icmp.ParseMessage needs for ICMPv4.
const icmpProtocol = 1

// errAllUnreachable reports that every probe failed with "no route to host",
// which on macOS means the terminal application lacks Local Network permission.
var errAllUnreachable = errors.New("every probe failed with no route to host")

// host is one live address and the evidence that proved it up.
type host struct {
	IP   net.IP
	RTT  time.Duration
	Via  []string
	Name string
	MAC  string
}

// icmpConn is the part of *icmp.PacketConn the sweep uses, so tests can fake it.
type icmpConn interface {
	WriteTo(b []byte, dst net.Addr) (int, error)
	ReadFrom(b []byte) (int, net.Addr, error)
	SetReadDeadline(t time.Time) error
	Close() error
}

// sweeper probes one subnet. Its listen and dial fields default to the real
// network and are replaced by fakes in tests.
type sweeper struct {
	subnet *net.IPNet
	local  net.IP
	ports  []int
	wait   time.Duration
	listen func() (icmpConn, error)
	dial   func(addr string, timeout time.Duration) (net.Conn, error)
	stderr io.Writer
}

// sweepReport is what one sweep produced before names and MACs are attached.
type sweepReport struct {
	hosts   []host
	fdFails int // dials that failed for lack of file descriptors
}

// newSweeper prepares a sweep of subnet that skips local and probes ports.
func newSweeper(subnet *net.IPNet, local net.IP, ports []int, wait time.Duration, stderr io.Writer) *sweeper {
	return &sweeper{
		subnet: subnet,
		local:  local,
		ports:  ports,
		wait:   wait,
		stderr: stderr,
		listen: func() (icmpConn, error) {
			c, err := icmp.ListenPacket("udp4", "0.0.0.0")
			if err != nil {
				return nil, err
			}
			return c, nil
		},
		dial: func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("tcp4", addr, timeout)
		},
	}
}

// evidence collects proof per address. The first proof fixes the RTT and later
// proofs only add their label.
type evidence struct {
	mu    sync.Mutex
	hosts map[string]*host
}

func newEvidence() *evidence { return &evidence{hosts: map[string]*host{}} }

// mark records that ip answered, proved by label after rtt.
func (e *evidence) mark(ip net.IP, rtt time.Duration, label string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	k := ip.String()
	h, ok := e.hosts[k]
	if !ok {
		e.hosts[k] = &host{IP: ip, RTT: rtt, Via: []string{label}}
		return
	}
	if !slices.Contains(h.Via, label) {
		h.Via = append(h.Via, label)
	}
}

// list returns a copy of every host with evidence.
func (e *evidence) list() []host {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]host, 0, len(e.hosts))
	for _, h := range e.hosts {
		out = append(out, *h)
	}
	return out
}

// run probes every address in the subnet and reports the hosts that answered.
func (s *sweeper) run() (sweepReport, error) {
	var targets []net.IP
	for _, ip := range subnetHosts(s.subnet) {
		if !ip.Equal(s.local) {
			targets = append(targets, ip)
		}
	}
	ev := newEvidence()
	var attempts, unreachable, fdFails atomic.Int64

	conn, err := s.listen()
	if err != nil {
		fmt.Fprintf(s.stderr, "%s: cannot open an ICMP socket (%v); probing with TCP only. On Linux, allow unprivileged ping with: sysctl -w net.ipv4.ping_group_range=\"0 2147483647\"\n", programName, err)
		conn = nil
	}
	readerDone := make(chan struct{})
	if conn == nil {
		close(readerDone)
	} else {
		sent := map[string]time.Time{}
		var sentMu sync.Mutex
		go func() {
			defer close(readerDone)
			buf := make([]byte, 1500)
			for {
				n, peer, err := conn.ReadFrom(buf)
				if err != nil {
					return
				}
				m, err := icmp.ParseMessage(icmpProtocol, buf[:n])
				if err != nil || m.Type != ipv4.ICMPTypeEchoReply {
					continue
				}
				ip := peerIP(peer)
				if ip == nil {
					continue
				}
				sentMu.Lock()
				at, ok := sent[ip.String()]
				sentMu.Unlock()
				if ok {
					ev.mark(ip, time.Since(at), "icmp")
				}
			}
		}()
		id := os.Getpid() & 0xffff
		for i, ip := range targets {
			msg := icmp.Message{Type: ipv4.ICMPTypeEcho, Code: 0, Body: &icmp.Echo{ID: id, Seq: i & 0xffff, Data: []byte(programName)}}
			b, err := msg.Marshal(nil)
			if err != nil {
				continue
			}
			sentMu.Lock()
			sent[ip.String()] = time.Now()
			sentMu.Unlock()
			attempts.Add(1)
			if _, err := conn.WriteTo(b, &net.UDPAddr{IP: ip}); err != nil && errors.Is(err, syscall.EHOSTUNREACH) {
				unreachable.Add(1)
			}
		}
		_ = conn.SetReadDeadline(time.Now().Add(s.wait))
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxInFlight)
	for _, ip := range targets {
		for _, p := range s.ports {
			wg.Add(1)
			sem <- struct{}{}
			go func(ip net.IP, p int) {
				defer wg.Done()
				defer func() { <-sem }()
				addr := net.JoinHostPort(ip.String(), strconv.Itoa(p))
				start := time.Now()
				attempts.Add(1)
				c, err := s.dial(addr, tcpTimeout)
				switch {
				case err == nil:
					_ = c.Close()
					ev.mark(ip, time.Since(start), "tcp"+strconv.Itoa(p))
				case errors.Is(err, syscall.ECONNREFUSED):
					ev.mark(ip, time.Since(start), "rst")
				case errors.Is(err, syscall.EMFILE), errors.Is(err, syscall.ENFILE):
					fdFails.Add(1)
				case errors.Is(err, syscall.EHOSTUNREACH):
					unreachable.Add(1)
				}
			}(ip, p)
		}
	}
	wg.Wait()
	<-readerDone
	if conn != nil {
		_ = conn.Close()
	}

	if a := attempts.Load(); a > 0 && unreachable.Load() == a {
		return sweepReport{}, errAllUnreachable
	}
	hosts := ev.list()
	if s.local != nil && s.subnet.Contains(s.local) {
		hosts = append(hosts, host{IP: s.local.To4(), Via: []string{"self"}})
	}
	sortHosts(hosts)
	return sweepReport{hosts: hosts, fdFails: int(fdFails.Load())}, nil
}

// peerIP extracts the IPv4 address an ICMP reply came from.
func peerIP(a net.Addr) net.IP {
	switch v := a.(type) {
	case *net.UDPAddr:
		return v.IP.To4()
	case *net.IPAddr:
		return v.IP.To4()
	}
	return nil
}

// parseCIDR accepts an IPv4 CIDR no wider than /16.
func parseCIDR(s string) (*net.IPNet, error) {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		return nil, fmt.Errorf("%s: %q is not a CIDR such as 192.168.1.0/24", programName, s)
	}
	return checkSubnet(n)
}

// checkSubnet rejects IPv6 subnets and prefixes wider than /16, returning the
// subnet in four-byte form.
func checkSubnet(n *net.IPNet) (*net.IPNet, error) {
	ip4 := n.IP.To4()
	if ip4 == nil {
		return nil, fmt.Errorf("%s: %s is IPv6; only IPv4 subnets are swept", programName, n)
	}
	ones, bits := n.Mask.Size()
	if bits == 128 {
		ones -= 96
	}
	if ones < minPrefix {
		return nil, fmt.Errorf("%s: %s is wider than the /%d limit; pass a narrower CIDR", programName, n, minPrefix)
	}
	mask := net.CIDRMask(ones, 32)
	return &net.IPNet{IP: ip4.Mask(mask), Mask: mask}, nil
}

// subnetHosts lists every host address in n, excluding the network and
// broadcast addresses.
func subnetHosts(n *net.IPNet) []net.IP {
	base := n.IP.Mask(n.Mask).To4()
	ones, bits := n.Mask.Size()
	if base == nil || bits != 32 {
		return nil
	}
	size := 1 << uint(bits-ones)
	if size <= 2 {
		return nil
	}
	start := uint32(base[0])<<24 | uint32(base[1])<<16 | uint32(base[2])<<8 | uint32(base[3])
	out := make([]net.IP, 0, size-2)
	for i := 1; i < size-1; i++ {
		v := start + uint32(i)
		out = append(out, net.IPv4(byte(v>>24), byte(v>>16), byte(v>>8), byte(v)).To4())
	}
	return out
}

// defaultSubnet returns the IPv4 subnet and address of the interface that
// routes to the internet, found from a UDP socket connected to a public
// address; no packet is sent.
func defaultSubnet() (*net.IPNet, net.IP, error) {
	c, err := net.Dial("udp4", "8.8.8.8:53")
	if err != nil {
		return nil, nil, err
	}
	local := c.LocalAddr().(*net.UDPAddr).IP.To4()
	_ = c.Close()
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, nil, err
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok && n.IP.Equal(local) {
				return &net.IPNet{IP: n.IP.Mask(n.Mask), Mask: n.Mask}, local, nil
			}
		}
	}
	return nil, nil, fmt.Errorf("no interface holds %s", local)
}

// resolveNames fills each host's reverse-DNS name, looking them up in parallel.
func resolveNames(hosts []host, lookup func(ctx context.Context, addr string) ([]string, error)) {
	var wg sync.WaitGroup
	for i := range hosts {
		wg.Add(1)
		go func(h *host) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), dnsTimeout)
			defer cancel()
			names, err := lookup(ctx, h.IP.String())
			if err == nil && len(names) > 0 {
				h.Name = strings.TrimSuffix(names[0], ".")
			}
		}(&hosts[i])
	}
	wg.Wait()
}

// sortHosts orders hosts by numeric IPv4 address.
func sortHosts(hosts []host) {
	slices.SortFunc(hosts, func(a, b host) int { return bytes.Compare(a.IP.To4(), b.IP.To4()) })
}
