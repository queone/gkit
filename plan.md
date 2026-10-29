# gkit Plan

## Product Direction

A collection of small CLI utilities written in Go. Each utility is a single-purpose, composable tool — installable as a standalone binary via `go install`. The repo prioritizes correctness, stability, and low-friction install/use over feature breadth.

## Ideas To Explore

Ideas captured for future reference. A bullet list — each line starts with `- IE<N>: ` (sequential N) for stable references. Two kinds: (a) **idea IE** — `IE<N>: <one-liner>`, awaiting Director discussion and the AC-ceremony evaluation in `AGENTS.md` `### AC-First Workflow`; (b) **AC-pointer** — `IE<N>: <one-liner> → govna/ac<N>-<slug>.md`, pointing at a drafted AC stub not yet through critique. An idea entry that the Director sends to Draft converts to an AC-pointer at AC-draft time, keeping its `IE<N>` number. Remove entries when the idea is rejected, retired, or (for AC-pointers) the AC has shipped and its file deleted. Not a historical record.

- IE4: Probe with ICMP in lslan on Windows, which has no unprivileged ICMP socket, so it currently falls back to TCP-only probing there.
- IE9: Make mdv links and images work in browsers that refuse `file://` URLs from a local page, such as DuckDuckGo. mdv sets `<base href="file:///<source-dir>/">`, so every relative link and image resolves to a `file://` URL. Options: open pages in a chosen browser that allows `file://` (a flag or config file; `open -a NAME` on macOS), keeping the default browser when unset; inline local images as `data:` URIs; render linked local `.md` files as sibling pages; serve pages from a loopback-only server, which would drop mdv's no-server promise.
