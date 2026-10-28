package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/hex"
	"regexp"
	"strings"
	"sync"
)

//go:generate go run oui_gen.go

// ouiData is the IEEE MA-L registry, one PREFIX<TAB>VENDOR line per prefix.
//
//go:embed oui.tsv.gz
var ouiData []byte

// ouiTable maps an upper-case six-hex-digit prefix to its vendor, loaded on
// first use.
var ouiTable = sync.OnceValue(func() map[string]string {
	t := map[string]string{}
	zr, err := gzip.NewReader(bytes.NewReader(ouiData))
	if err != nil {
		return t
	}
	sc := bufio.NewScanner(zr)
	for sc.Scan() {
		if prefix, name, ok := strings.Cut(sc.Text(), "\t"); ok {
			t[prefix] = name
		}
	}
	return t
})

// vendor names the maker of mac from the registry: "private" for a locally
// administered (randomized) address, or "" for an empty, malformed, or
// unknown one.
func vendor(mac string) string {
	norm, ok := normalizeMAC(mac)
	if !ok {
		return ""
	}
	b, err := hex.DecodeString(strings.ReplaceAll(norm[:8], ":", ""))
	if err != nil {
		return ""
	}
	if b[0]&0x02 != 0 {
		return "private"
	}
	return ouiTable()[strings.ToUpper(hex.EncodeToString(b))]
}

// appleModels are the device-info model prefixes Apple devices publish.
var appleModels = []string{"Mac", "iPhone", "iPad", "iPod", "AppleTV", "AudioAccessory", "Watch"}

// appleModel reports whether a device-info model names an Apple device: a
// known family prefix, or a board ID such as J255AP.
func appleModel(model string) bool {
	for _, p := range appleModels {
		if strings.HasPrefix(model, p) {
			return true
		}
	}
	return len(model) > 2 && strings.HasSuffix(model, "AP")
}

// iosDHCPName matches the generic DHCP host name iOS sends with a private
// address, as a router publishes it: iPhone or iPad, maybe with a -N suffix.
var iosDHCPName = regexp.MustCompile(`(?i)^(iphone|ipad)(-[0-9]+)?$`)

// vendorFor names the maker of mac. An Apple signal, a device-info model or
// a DNS name before any mDNS name that is the generic iOS DHCP host name,
// refines a randomized MAC to "private (Apple)" and gives "Apple" when the
// MAC is unknown, as it is without sudo on macOS 27.
func vendorFor(mac, model, dnsName string) string {
	label, _, _ := strings.Cut(dnsName, ".")
	apple := appleModel(model) || iosDHCPName.MatchString(label)
	if mac == "" {
		if apple {
			return "Apple"
		}
		return ""
	}
	v := vendor(mac)
	if v == "private" && apple {
		return "private (Apple)"
	}
	return v
}
