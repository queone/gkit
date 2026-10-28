package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"regexp"
	"testing"
)

func TestVendorNamesTheMaker(t *testing.T) {
	for _, tc := range []struct{ mac, want string }{
		{"00:00:00:12:34:56", "XEROX CORPORATION"},
		{"00:0:0:12:34:56", "XEROX CORPORATION"},
		{"AC:DE:48:00:11:22", "Private"}, // the registry's own "Private" MA-L entry
		{"da:a1:19:00:00:01", "private"},
		{"", ""},
		{"not-a-mac", ""},
		{"00:00:00:12:34", ""},
	} {
		if got := vendor(tc.mac); got != tc.want {
			t.Errorf("vendor(%q) = %q, want %q", tc.mac, got, tc.want)
		}
	}
	upper, lower := vendor("3C:22:FB:00:00:01"), vendor("3c:22:fb:00:00:01")
	if upper == "" || upper != lower {
		t.Errorf("vendor upper %q and lower %q, want the same known vendor", upper, lower)
	}
	if v := vendor("02:00:00:00:00:01"); v != "private" {
		t.Errorf("locally administered vendor = %q, want private", v)
	}
	if v := vendor("fc:ff:ff:00:00:01"); v != "" {
		t.Errorf("unknown prefix vendor = %q, want empty", v)
	}
}

func TestEmbeddedRegistryIsCompleteAndSorted(t *testing.T) {
	zr, err := gzip.NewReader(bytes.NewReader(ouiData))
	if err != nil {
		t.Fatal(err)
	}
	line := regexp.MustCompile(`^[0-9A-F]{6}\t\S.*$`)
	sc := bufio.NewScanner(zr)
	var n int
	var prev string
	for sc.Scan() {
		l := sc.Text()
		if !line.MatchString(l) {
			t.Fatalf("line %d %q: want PREFIX<TAB>VENDOR", n+1, l)
		}
		if l[:6] <= prev {
			t.Fatalf("line %d prefix %s follows %s, want sorted unique prefixes", n+1, l[:6], prev)
		}
		prev = l[:6]
		n++
	}
	if n <= 30000 {
		t.Fatalf("registry holds %d prefixes, want more than 30000", n)
	}
}

func TestVendorForRefinesOnlyRandomizedMACs(t *testing.T) {
	for _, tc := range []struct{ mac, model, dnsName, want string }{
		{"ee:5e:8a:d7:48:c6", "Mac16,11", "", "private (Apple)"},
		{"b2:b0:5c:16:8e:6d", "iPhone15,2", "", "private (Apple)"},
		{"b2:b0:5c:16:8e:6d", "J255AP", "", "private (Apple)"},
		{"b2:b0:5c:16:8e:6d", "Roku3,1", "", "private"},
		{"b2:b0:5c:16:8e:6d", "", "", "private"},
		{"b2:b0:5c:16:8e:6d", "", "iPhone.lan", "private (Apple)"},
		{"b2:b0:5c:16:8e:6d", "", "ipad-2.lan", "private (Apple)"},
		{"b2:b0:5c:16:8e:6d", "", "iPhone", "private (Apple)"},
		{"b2:b0:5c:16:8e:6d", "", "iPhoneish.lan", "private"},
		{"b2:b0:5c:16:8e:6d", "", "myiphone.lan", "private"},
		{"18:a5:ff:9d:6e:24", "Mac16,11", "iPhone.lan", "Arcadyan Corporation"},
		{"", "Mac16,11", "", "Apple"},
		{"", "", "iPhone.lan", "Apple"},
		{"", "Roku3,1", "router.lan", ""},
		{"", "", "", ""},
	} {
		if got := vendorFor(tc.mac, tc.model, tc.dnsName); got != tc.want {
			t.Errorf("vendorFor(%q, %q, %q) = %q, want %q", tc.mac, tc.model, tc.dnsName, got, tc.want)
		}
	}
}
