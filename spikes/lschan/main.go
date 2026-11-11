// Throwaway test for the lschan idea in plan.md: can a plain Go binary, built
// without cgo, read per-network Wi-Fi signal from CoreWLAN through purego,
// without Location Services permission? On macOS 27 it can.
//
// This folder is its own Go module, so ./build.sh and the root go.mod ignore it.
//
// Run from this folder:
//
//	CGO_ENABLED=0 go run . [SCANS GAP]
//
// SCANS defaults to 3 and GAP to 0, such as `go run . 6 10s`. A scan started
// right after another returns CoreWLAN's cache instead of rescanning. Each
// network prints as one JSON line on stdout; the interface and per-scan
// timings print on stderr. scans-back-to-back.jsonl and scans-10s-gap.jsonl
// hold the recorded runs, and analyze.py scores channels from either one.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

func init() { runtime.LockOSThread() }

type row struct {
	Scan     int    `json:"scan"`
	Band     int    `json:"band"`
	Channel  int    `json:"channel"`
	Width    int    `json:"width"`
	RSSI     int    `json:"rssi"`
	Noise    int    `json:"noise"`
	SSID     string `json:"ssid"`
	BSSID    string `json:"bssid"`
	IELen    int    `json:"ie_len"`
	IESSID   string `json:"ie_ssid,omitempty"`
	LoadUtil int    `json:"load_util"` // -1 when no BSS Load element
	Beacon   int    `json:"beacon"`
}

func cstr(p uintptr) string {
	if p == 0 {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice((*byte)(unsafe.Pointer(p)), n))
}

var (
	sUTF8   = objc.RegisterName("UTF8String")
	sLength = objc.RegisterName("length")
	sBytes  = objc.RegisterName("bytes")
)

func nsString(id objc.ID) string {
	if id == 0 {
		return "<nil>"
	}
	return cstr(objc.Send[uintptr](id, sUTF8))
}

func nsData(id objc.ID) []byte {
	if id == 0 {
		return nil
	}
	n := objc.Send[uint](id, sLength)
	p := objc.Send[uintptr](id, sBytes)
	if n == 0 || p == 0 {
		return nil
	}
	return append([]byte(nil), unsafe.Slice((*byte)(unsafe.Pointer(p)), n)...)
}

// parseIE returns the SSID element text and BSS Load channel utilization (0-255, -1 if absent).
func parseIE(b []byte) (string, int) {
	ssid, util := "", -1
	for i := 0; i+2 <= len(b); {
		id, l := b[i], int(b[i+1])
		if i+2+l > len(b) {
			break
		}
		v := b[i+2 : i+2+l]
		switch id {
		case 0:
			ssid = string(v)
		case 11:
			if l >= 3 {
				util = int(v[2])
			}
		}
		i += 2 + l
	}
	return ssid, util
}

func main() {
	if _, err := purego.Dlopen("/System/Library/Frameworks/CoreWLAN.framework/CoreWLAN", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		fmt.Fprintln(os.Stderr, "dlopen CoreWLAN:", err)
		os.Exit(1)
	}
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))

	client := objc.ID(objc.GetClass("CWWiFiClient")).Send(objc.RegisterName("sharedWiFiClient"))
	iface := client.Send(objc.RegisterName("interface"))
	if iface == 0 {
		fmt.Fprintln(os.Stderr, "no Wi-Fi interface")
		os.Exit(1)
	}
	sChan := objc.RegisterName("wlanChannel")
	sNum := objc.RegisterName("channelNumber")
	sWidth := objc.RegisterName("channelWidth")
	sBand := objc.RegisterName("channelBand")
	sRSSI := objc.RegisterName("rssiValue")
	sNoise := objc.RegisterName("noiseMeasurement")
	sSSID := objc.RegisterName("ssid")
	sBSSID := objc.RegisterName("bssid")
	sIE := objc.RegisterName("informationElementData")
	sBeacon := objc.RegisterName("beaconInterval")

	ich := iface.Send(sChan)
	fmt.Fprintf(os.Stderr, "iface=%s ssid=%s bssid=%s rssi=%d noise=%d chan=%d width=%d band=%d\n",
		nsString(iface.Send(objc.RegisterName("interfaceName"))),
		nsString(iface.Send(sSSID)), nsString(iface.Send(sBSSID)),
		objc.Send[int](iface, sRSSI), objc.Send[int](iface, sNoise),
		objc.Send[int](ich, sNum), objc.Send[int](ich, sWidth), objc.Send[int](ich, sBand))

	sScan := objc.RegisterName("scanForNetworksWithName:error:")
	sAll := objc.RegisterName("allObjects")
	sCount := objc.RegisterName("count")
	sAt := objc.RegisterName("objectAtIndex:")
	enc := json.NewEncoder(os.Stdout)
	scans, gap := 3, time.Duration(0)
	if len(os.Args) > 2 {
		fmt.Sscan(os.Args[1], &scans)
		gap, _ = time.ParseDuration(os.Args[2])
	}
	for scan := 1; scan <= scans; scan++ {
		if scan > 1 {
			time.Sleep(gap)
		}
		var nsErr objc.ID
		t0 := time.Now()
		set := iface.Send(sScan, objc.ID(0), unsafe.Pointer(&nsErr))
		el := time.Since(t0)
		if set == 0 {
			fmt.Fprintf(os.Stderr, "scan %d failed after %v: %s\n", scan, el,
				nsString(nsErr.Send(objc.RegisterName("localizedDescription"))))
			continue
		}
		arr := set.Send(sAll)
		n := objc.Send[uint](arr, sCount)
		fmt.Fprintf(os.Stderr, "scan %d: %d networks in %v\n", scan, n, el.Round(time.Millisecond))
		for i := uint(0); i < n; i++ {
			nw := arr.Send(sAt, i)
			ch := nw.Send(sChan)
			ie := nsData(nw.Send(sIE))
			ies, util := parseIE(ie)
			enc.Encode(row{
				Scan: scan, Band: objc.Send[int](ch, sBand), Channel: objc.Send[int](ch, sNum),
				Width: objc.Send[int](ch, sWidth), RSSI: objc.Send[int](nw, sRSSI),
				Noise: objc.Send[int](nw, sNoise), SSID: nsString(nw.Send(sSSID)),
				BSSID: nsString(nw.Send(sBSSID)), IELen: len(ie), IESSID: ies,
				LoadUtil: util, Beacon: objc.Send[int](nw, sBeacon),
			})
		}
	}
}
