# gkit Plan

## Product Direction

A collection of small CLI utilities written in Go. Each utility is a single-purpose, composable tool — installable as a standalone binary via `go install`. The repo prioritizes correctness, stability, and low-friction install/use over feature breadth.

## Ideas To Explore

Ideas captured for future reference. A bullet list — each line starts with `- IE<N>: ` (sequential N) for stable references. Two kinds: (a) **pre-rubric IE** — `IE<N>: <one-liner>`, awaiting director discussion and the objective-fit rubric (see `AGENTS.md` Approval Boundaries); (b) **AC-pointer** — `IE<N>: <one-liner> → docs/ac<N>-<slug>.md`, pointing at a drafted AC stub not yet through critique. A pre-rubric entry that clears the rubric converts to an AC-pointer at AC-draft time, keeping its `IE<N>` number. Remove entries when the idea is rejected, retired, or (for AC-pointers) the AC has shipped and its file deleted. Not a historical record.

- IE1: Standardize the alias form in help `Commands` rows; today attune writes `(c|validate)`, repoctl `s, status`, and swatch `p, palette`.
- IE2: Add a README to the ten utilities without one (bak, brew-update, cash5, certgen, certls, dl, dos2unix, pman, rncap, rnlower), then make build.sh fail on any `cmd/<name>` without one.
- IE3: Show vendor names in lslan from an OUI table when a MAC address is present.
- IE4: Probe with ICMP in lslan on Windows, which has no unprivileged ICMP socket, so it currently falls back to TCP-only probing there.
- IE5: Add `-s, --server ADDR` to lslan so reverse-DNS lookups go to a named server, since a VPN resolver ahead of the router hides the router's names.
- IE6: Fall back in lslan to the subnet's default gateway for reverse-DNS when the system resolver has no name; needs a per-platform route lookup and assumes the gateway serves DNS.
