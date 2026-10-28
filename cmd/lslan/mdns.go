package main

import (
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

// mdnsGroup is the multicast DNS group and port.
const mdnsGroup = "224.0.0.251:5353"

// mdnsWait is how long the mDNS step collects answers.
const mdnsWait = 500 * time.Millisecond

// mdnsInfo is what a host says about itself over multicast DNS.
type mdnsInfo struct {
	Name  string // the host's own .local name, without the trailing dot
	Model string // the device-info model, such as Mac16,11, or ""
}

// mdnsLookup asks group for the .local name of every address in ips, then for
// the device-info model of every name that answers, all within one wait. It
// uses one unconnected socket, bound to local when known, because each device
// answers from its own address. It returns an empty map on any socket error.
func mdnsLookup(local net.IP, group string, ips []net.IP, wait time.Duration) map[string]mdnsInfo {
	out := map[string]mdnsInfo{}
	if len(ips) == 0 {
		return out
	}
	gaddr, err := net.ResolveUDPAddr("udp4", group)
	if err != nil {
		return out
	}
	laddr := &net.UDPAddr{}
	if ip4 := local.To4(); ip4 != nil {
		laddr.IP = ip4
	}
	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return out
	}
	defer conn.Close()

	send := func(name string, typ dnsmessage.Type) {
		if msg, err := mdnsQuery(name, typ); err == nil {
			conn.WriteToUDP(msg, gaddr)
		}
	}
	reverse := map[string]string{} // lower-case in-addr.arpa name -> IP
	for _, ip := range ips {
		ip4 := ip.To4()
		if ip4 == nil {
			continue
		}
		name := fmt.Sprintf("%d.%d.%d.%d.in-addr.arpa.", ip4[3], ip4[2], ip4[1], ip4[0])
		reverse[name] = ip4.String()
		send(name, dnsmessage.TypePTR)
	}
	devInfo := map[string]string{} // lower-case device-info name -> IP

	if err := conn.SetReadDeadline(time.Now().Add(wait)); err != nil {
		return out
	}
	buf := make([]byte, 9000)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			return out
		}
		for _, r := range mdnsRecords(buf[:n]) {
			owner := strings.ToLower(r.Header.Name.String())
			switch body := r.Body.(type) {
			case *dnsmessage.PTRResource:
				ip, ok := reverse[owner]
				if !ok || out[ip].Name != "" {
					continue
				}
				host := strings.TrimSuffix(body.PTR.String(), ".")
				info := out[ip]
				info.Name = host
				out[ip] = info
				if label, ok := strings.CutSuffix(strings.ToLower(host), ".local"); ok && label != "" && !strings.Contains(label, ".") {
					name := strings.Split(host, ".")[0] + "._device-info._tcp.local."
					devInfo[strings.ToLower(name)] = ip
					send(name, dnsmessage.TypeTXT)
				}
			case *dnsmessage.TXTResource:
				ip, ok := devInfo[owner]
				if !ok {
					continue
				}
				for _, kv := range body.TXT {
					if model, ok := strings.CutPrefix(kv, "model="); ok {
						info := out[ip]
						info.Model = model
						out[ip] = info
					}
				}
			}
		}
	}
}

// mdnsQuery builds one question for name and typ.
func mdnsQuery(name string, typ dnsmessage.Type) ([]byte, error) {
	n, err := dnsmessage.NewName(name)
	if err != nil {
		return nil, err
	}
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{})
	if err := b.StartQuestions(); err != nil {
		return nil, err
	}
	if err := b.Question(dnsmessage.Question{Name: n, Type: typ, Class: dnsmessage.ClassINET}); err != nil {
		return nil, err
	}
	return b.Finish()
}

// mdnsRecords returns the answer and additional records of one reply, or none
// when the reply does not parse.
func mdnsRecords(msg []byte) []dnsmessage.Resource {
	var p dnsmessage.Parser
	if _, err := p.Start(msg); err != nil {
		return nil
	}
	if err := p.SkipAllQuestions(); err != nil {
		return nil
	}
	answers, err := p.AllAnswers()
	if err != nil {
		return nil
	}
	if err := p.SkipAllAuthorities(); err != nil {
		return answers
	}
	extra, _ := p.AllAdditionals()
	return append(answers, extra...)
}
