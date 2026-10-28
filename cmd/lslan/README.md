## lslan
List the live hosts on the local network in about a second, without sudo.

### Why
macOS 27 stopped letting ordinary processes read the ARP table, the kernel's map from IP address to MAC address. Discovery tools that depend on that table now list only the devices that announce themselves over mDNS or SSDP, so a quiet device such as a thermostat, a printer with discovery turned off, or a phone in power-save mode vanishes from the list even though it answers ping. `lslan` does not need the table to find hosts: it asks every address in the subnet directly and prints whatever answered.

```bash
lslan
IP              RTT  VIA                NAME          MAC  VENDOR
192.168.12.1    15   icmp,tcp80         TMO-G4AR.lan
192.168.12.140  0    self
192.168.12.141  255  icmp               Mac.lan
192.168.12.151  255  icmp               np05.lan
192.168.12.187  444  tcp80,icmp,tcp443
```

On a terminal the columns are aligned under a header. Piped output is tab-separated with no header, one host per line, and the summary line goes to stderr, so stdout stays a clean list for scripts:

```text
5 hosts on 192.168.12.0/24 in 853 ms
```

### How it finds hosts
- One ICMP echo request to every address in the subnet, sent from an unprivileged datagram socket, so no root and no raw sockets.
- One TCP connection attempt per address to ports 22, 80 and 443. An open port proves the host is up, and so does a refused connection, since only a running TCP stack sends a reset. A timeout proves nothing.
- A reverse-DNS lookup for every host that answered, unless `-n` is given. See **Names behind a VPN** for which server answers.
- The local host itself appears with `self` as its evidence.

A /24 finishes in about a second. The widest subnet accepted is a /16. A host that drops ping and has nothing listening on the probed ports stays invisible; pass other ports with `-p` when you know what a device listens on.

### Names behind a VPN
A VPN usually replaces the system resolver with its own, which knows nothing about the hosts on your LAN, so the NAME column comes back empty. `lslan` works around that in two ways:

- Without `-s`, it asks the system resolver first, then asks the default gateway of the swept subnet for every host still unnamed. Most home routers answer for the devices they hand addresses to. A gateway that serves no DNS adds up to the 500 ms lookup timeout.
- Without `-s`, it then asks multicast DNS (mDNS, the Bonjour lookup) about every host. A host that answers gives its own `.local` name, the one its owner set, such as `np17.local` for an iPhone, and that name replaces the router's generic one. Hosts that do not answer, such as most routers, keep their DNS name. This step waits 500 ms on every run.
- With `-s ADDR`, it sends every lookup to ADDR alone and skips mDNS, such as `lslan -s 192.168.1.1`. ADDR is an IPv4 address with an optional `:PORT`; the port defaults to 53. `-s` and `-n` conflict.

### MAC addresses and sudo
After probing, `lslan` reads the system neighbor table and fills the MAC column for every host it found. On macOS 27 only root may read that table, so the column stays empty unless you run `sudo lslan`. On Linux any user can read it. `lslan` never tries to work around the macOS restriction.

The VENDOR column names each MAC's maker from the IEEE MA-L registry built into `lslan`, or shows `private` for a locally administered address, the randomized kind most phones now use. A randomized MAC cannot name its maker, so when the host's mDNS device-info model is an Apple one, such as `Mac16,11`, VENDOR shows `private (Apple)` instead. An idle iPhone publishes no model, but with a private address iOS sends the generic DHCP host name `iPhone` (or `iPad`), so a router name such as `iPhone.lan` also gives `private (Apple)`. Both Apple signals work without the MAC, so without sudo VENDOR still shows `Apple` for those hosts and stays empty for the rest. VENDOR is the sixth tab-separated column, after MAC, so a script that reads five fields gets MAC and VENDOR together. To refresh the registry, run `go generate` in `cmd/lslan`, which downloads the IEEE file and rewrites `oui.tsv.gz`.

### Local Network permission
macOS grants access to the local subnet per application. If every probe fails with "no route to host", allow your terminal application under System Settings > Privacy & Security > Local Network and run `lslan` again.

### Getting Started
This utility is part of a collection of Go utilities. To compile and install follow the **Getting Started** instructions at the [gkit repo](https://github.com/queone/gkit).

### Usage

```text
lslan v1.2.0
List the live hosts on the local network
github.com/queone/gkit/tree/main/cmd/lslan

Usage
  lslan [OPTIONS] [CIDR]  List the hosts that answer on the local subnet, or on CIDR

  Without CIDR, lslan sweeps the IPv4 subnet of the interface that routes to
  the internet. A prefix wider than /16 is rejected; pass a narrower CIDR.
  A terminal gets aligned columns under a header; piped output is
  tab-separated with no header.

Options
  -n, --numeric      Skip reverse-DNS lookups and leave NAME empty
  -j, --json         Print a JSON array instead of tab-separated lines
  -w, --wait MS      Wait MS milliseconds for late ICMP replies; default 500
  -p, --ports LIST   Probe these TCP ports; default 22,80,443, and an empty LIST disables them
  -s, --server ADDR  Send every reverse-DNS lookup to ADDR, an IPv4 address with an optional :PORT
  -v, --version      Print lslan v1.2.0 and exit
  -h, -?, --help     Show this help and exit

Columns
  IP      IPv4 address, in numeric order
  RTT     Milliseconds until the first proof that the host is up
  VIA     Evidence: icmp, tcpNN for an open port, rst for a closed one, self
  NAME    Name from -s alone; otherwise mDNS, else the system resolver, else the gateway
  MAC     From the neighbor table when readable; macOS 27 needs sudo
  VENDOR  Maker from the IEEE registry; private (Apple) when randomized; Apple without a MAC

Examples
  lslan                 List the local subnet
  lslan -n 10.0.5.0/24  List another subnet without name lookups
  lslan -s 192.168.1.1  Ask the router for names that a VPN resolver hides
  sudo lslan -j | jq .  JSON with MAC addresses on macOS
```
