package main

import (
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

// mdnsResponder answers on a local UDP port like an mDNS group: names maps an
// in-addr.arpa name to a .local name, models maps a device-info name to a
// model, and every reply also carries a stray PTR for an address not asked.
func mdnsResponder(t *testing.T, names, models map[string]string) string {
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
			if _, err := p.Start(buf[:n]); err != nil {
				continue
			}
			q, err := p.Question()
			if err != nil {
				continue
			}
			b := dnsmessage.NewBuilder(nil, dnsmessage.Header{Response: true, Authoritative: true})
			_ = b.StartAnswers()
			hdr := func(name string, typ dnsmessage.Type) dnsmessage.ResourceHeader {
				return dnsmessage.ResourceHeader{Name: dnsmessage.MustNewName(name), Type: typ, Class: dnsmessage.ClassINET, TTL: 120}
			}
			_ = b.PTRResource(hdr("99.0.0.10.in-addr.arpa.", dnsmessage.TypePTR), dnsmessage.PTRResource{PTR: dnsmessage.MustNewName("stray.local.")})
			qn := strings.ToLower(q.Name.String())
			if host, ok := names[qn]; ok && q.Type == dnsmessage.TypePTR {
				_ = b.PTRResource(hdr(q.Name.String(), dnsmessage.TypePTR), dnsmessage.PTRResource{PTR: dnsmessage.MustNewName(host)})
			}
			if model, ok := models[qn]; ok && q.Type == dnsmessage.TypeTXT {
				_ = b.TXTResource(hdr(q.Name.String(), dnsmessage.TypeTXT), dnsmessage.TXTResource{TXT: []string{"model=" + model, "osxvers=26"}})
			}
			if msg, err := b.Finish(); err == nil {
				pc.WriteTo(msg, from)
			}
		}
	}()
	return pc.LocalAddr().String()
}

func TestMDNSLookupNamesAndModels(t *testing.T) {
	group := mdnsResponder(t,
		map[string]string{"7.0.0.10.in-addr.arpa.": "np17.local.", "8.0.0.10.in-addr.arpa.": "Denon-AVR.local."},
		map[string]string{"np17._device-info._tcp.local.": "iPhone15,2", "stray._device-info._tcp.local.": "Mac1,1"})
	ips := []net.IP{net.IPv4(10, 0, 0, 7), net.IPv4(10, 0, 0, 8), net.IPv4(10, 0, 0, 9)}
	got := mdnsLookup(net.IPv4(127, 0, 0, 1), group, ips, 300*time.Millisecond)
	want := map[string]mdnsInfo{
		"10.0.0.7": {Name: "np17.local", Model: "iPhone15,2"},
		"10.0.0.8": {Name: "Denon-AVR.local"},
	}
	if len(got) != len(want) {
		t.Fatalf("mdnsLookup = %+v, want %+v", got, want)
	}
	for ip, w := range want {
		if got[ip] != w {
			t.Errorf("mdnsLookup[%s] = %+v, want %+v", ip, got[ip], w)
		}
	}
}

func TestMDNSLookupReturnsEmptyWithinTheWait(t *testing.T) {
	silent, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer silent.Close()
	start := time.Now()
	got := mdnsLookup(nil, silent.LocalAddr().String(), []net.IP{net.IPv4(10, 0, 0, 7)}, 100*time.Millisecond)
	if len(got) != 0 {
		t.Errorf("mdnsLookup = %+v, want empty", got)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("mdnsLookup took %v, want about the 100ms wait", d)
	}
	if got := mdnsLookup(nil, silent.LocalAddr().String(), nil, time.Hour); len(got) != 0 {
		t.Errorf("mdnsLookup with no addresses = %+v, want empty", got)
	}
}
