## lslan
List the live hosts on the local network in about a second, without sudo.

### Why?
macOS 27 stopped letting ordinary processes read the ARP table, the kernel's map from IP address to MAC address. Discovery tools that depend on that table now list only the devices that announce themselves over mDNS or SSDP, so a quiet device such as a thermostat, a printer with discovery turned off, or a phone in power-save mode vanishes from the list even though it answers ping. `lslan` does not need the table to find hosts: it asks every address in the subnet directly and prints whatever answered.

```bash
lslan
192.168.12.1	15	icmp,tcp80		
192.168.12.140	0	self		
192.168.12.141	255	icmp		
192.168.12.151	255	icmp		
192.168.12.187	444	tcp80,icmp,tcp443		
```

The summary line goes to stderr, so stdout stays a clean list for scripts:

```text
5 hosts on 192.168.12.0/24 in 853 ms
```

### How it finds hosts
- One ICMP echo request to every address in the subnet, sent from an unprivileged datagram socket, so no root and no raw sockets.
- One TCP connection attempt per address to ports 22, 80 and 443. An open port proves the host is up, and so does a refused connection, since only a running TCP stack sends a reset. A timeout proves nothing.
- A reverse-DNS lookup for every host that answered, unless `-n` is given.
- The local host itself appears with `self` as its evidence.

A /24 finishes in about a second. The widest subnet accepted is a /16. A host that drops ping and has nothing listening on the probed ports stays invisible; pass other ports with `-p` when you know what a device listens on.

### MAC addresses and sudo
After probing, `lslan` reads the system neighbor table and fills the MAC column for every host it found. On macOS 27 only root may read that table, so the column stays empty unless you run `sudo lslan`. On Linux any user can read it. `lslan` never tries to work around the macOS restriction.

### Local Network permission
macOS grants access to the local subnet per application. If every probe fails with "no route to host", allow your terminal application under System Settings > Privacy & Security > Local Network and run `lslan` again.

### Getting Started
This utility is part of a collection of Go utilities. To compile and install follow the **Getting Started** instructions at the [gkit repo](https://github.com/queone/gkit).

### Usage

```text
lslan v1.0.0
List the live hosts on the local network
github.com/queone/gkit/tree/main/cmd/lslan

Usage
  lslan [OPTIONS] [CIDR]  List the hosts that answer on the local subnet, or on CIDR

  Without CIDR, lslan sweeps the IPv4 subnet of the interface that routes to
  the internet. A prefix wider than /16 is rejected; pass a narrower CIDR.

Options
  -n, --numeric     Skip reverse-DNS lookups and leave NAME empty
  -j, --json        Print a JSON array instead of tab-separated lines
  -w, --wait MS     Wait MS milliseconds for late ICMP replies; default 500
  -p, --ports LIST  Probe these TCP ports; default 22,80,443, and an empty LIST disables them
  -v, --version     Print lslan v1.0.0 and exit
  -h, -?, --help    Show this help and exit

Columns
  IP    IPv4 address, in numeric order
  RTT   Milliseconds until the first proof that the host is up
  VIA   Evidence: icmp, tcpNN for an open port, rst for a closed one, self
  NAME  Reverse-DNS name, or empty
  MAC   From the neighbor table when readable; macOS 27 needs sudo

Examples
  lslan                 List the local subnet
  lslan -n 10.0.5.0/24  List another subnet without name lookups
  sudo lslan -j | jq .  JSON with MAC addresses on macOS
```
