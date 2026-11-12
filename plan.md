# gkit Plan

## Product Direction

A collection of small CLI utilities written in Go. Each utility is a single-purpose, composable tool — installable as a standalone binary via `go install`. The repo prioritizes correctness, stability, and low-friction install/use over feature breadth.

## Ideas To Explore

Ideas captured for future reference. A bullet list — each line starts with `- IE<N>: ` (sequential N) for stable references, and one blank line separates items. Two kinds: (a) **idea IE** — `IE<N>: <one-liner>`, awaiting Director discussion and the AC-ceremony evaluation in `AGENTS.md` `### AC-First Workflow`; (b) **AC-pointer** — `IE<N>: <one-liner> → govna/ac<N>-<slug>.md`, pointing at a drafted AC stub not yet through critique. An idea entry that the Director sends to Draft converts to an AC-pointer at AC-draft time, keeping its `IE<N>` number. Remove entries when the idea is rejected, retired, or (for AC-pointers) the AC has shipped and its file deleted. Not a historical record.

- IE4: Probe with ICMP in lslan on Windows, which has no unprivileged ICMP socket, so it currently falls back to TCP-only probing there.

- IE9: Make mdv links and images work in browsers that refuse `file://` URLs from a local page, such as DuckDuckGo. mdv sets `<base href="file:///<source-dir>/">`, so every relative link and image resolves to a `file://` URL. Options: open pages in a chosen browser that allows `file://` (a flag or config file; `open -a NAME` on macOS), keeping the default browser when unset; inline local images as `data:` URIs; render linked local `.md` files as sibling pages; serve pages from a loopback-only server, which would drop mdv's no-server promise.

- IE10: Add lschan, a macOS tool that ranks Wi-Fi channels by how crowded they are, modeled on wifiscan (github.com/lucasdaddiego/wifiscan) and named to pair with lslan. Read scans from CoreWLAN through purego, so the build needs no cgo. A test on macOS 27 returned signal in dBm, channel, width, and band for every nearby network without Location permission, sudo, or a prompt, but no names, router addresses, or information elements, so no load reports. The test code and its recorded scans live in `spikes/lschan`, a separate Go module that the build ignores. Score each channel by signal power weighted by spectrum overlap, as wifiscan does, and recommend a move only when another channel beats the current one by a clear margin, such as 6 dB; otherwise say to stay. Ship macOS only first. One fresh scan takes about 11 s; a scan right after another returns a stale cache, and a 10 s gap gave fresh results. Open decisions: skip the connected router automatically and offer a flag to skip more channels, since lschan cannot tell the user's other radios from a neighbor's (recommended), or show the per-channel view with no recommendation; default to 1 scan with a flag for more (recommended), or keep the earlier plan of averaging 3 scans, about 55 s.

- IE11: Scan Wi-Fi on Linux in lschan, which reads scans only through macOS CoreWLAN, with a Linux source such as `nmcli` or nl80211.

- IE12: Make lschan a periodic recommender: run it on a schedule, such as from a per-user launchd agent, log every scan, and recommend router settings for least interference, covering channel, channel width, and band. Base each recommendation on scans across hours and days, so a busy hour or one neighbor's channel change does not move it, and recommend a change only when it beats the current setup by a clear margin and holds over days. Test first whether CoreWLAN scans from a launchd agent; macwifi-cli reports that its scans fail from a system-wide launchd daemon. Open decision: whether the first lschan version already recommends width and band, or only channel.
