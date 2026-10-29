# gkit History

One section per release, newest first: what the release added, changed, removed, or deferred in the product, its design, and its code, and why. Governance, toolchain, and release mechanics stay in `CHANGELOG.md` and git. Settled design is in `arch.md`. Search it to learn why a release changed the product.

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
