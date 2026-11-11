# gkit History

One section per release, newest first: what the release added, changed, removed, or deferred in the product, its design, and its code, and why. Governance, toolchain, and release mechanics stay in `CHANGELOG.md` and git. Settled design is in `arch.md`. Search it to learn why a release changed the product.

## v0.96.2

Added:
- `spikes/lschan`, a test program that reads Wi-Fi scans from CoreWLAN through `purego`, built without cgo.
- Two recorded scan runs and `analyze.py`, which scores channel crowding from one run.

Deferred:
- lschan, a macOS tool that recommends Wi-Fi channel, width, and band, until its open decisions settle.
- Linux scans and periodic recommendations in lschan, until the first macOS version ships.

Decisions:
- lschan reads scans from CoreWLAN through `purego`, so it needs no cgo, no extra install, and no permission prompt.
- lschan scores channels by signal power weighted by spectrum overlap, the method that wifiscan uses.
- lschan recommends a move only when another channel wins by a clear margin, so scan noise does not move it.
- The test code is its own Go module, so the root `go.mod` gains no `purego` dependency and `./build.sh` skips it.

Risks:
- Without Location permission, CoreWLAN returns no network names or router addresses.
- So lschan cannot tell the user's own radios from a neighbor's radios, and can count them as interference.
- One fresh scan takes about 11 s, and a scan right after another returns a stale cache.
- `./build.sh` does not build, vet, or test the test code.

## v0.96.0

Changed:
- `update` records the running version in a new `current` field of its cache file.
- `update` treats a cache from an older version as empty, because a pre-upgrade cache reported an old release as the latest.
- `update` treats a cache with a missing or invalid `current` as empty.
- README and arch.md state that the update check also runs once after an upgrade.

Decisions:
- A cache from the same or a newer version stays in use, because the newest proxy release does not depend on the asker.

Risks:
- gkit utilities on different versions share one cache, so the newer one can ask the proxy once more per interval.
- Each existing cache file without `current` costs one extra proxy request.
