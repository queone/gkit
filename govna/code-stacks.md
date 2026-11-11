# First-Class CODE Stacks

- Use only Go, Rust, Swift, or Terraform as selectable CODE stacks.
- Use this reference for their selection, validation, installation, release prep, and scoped builds.

## Multi-Utility Versioning

- Apply `govna/build-release.md` `## Independent Utility Versions` to every installable utility.
- Let each stack adapter choose declaration syntax and source layout while reporting the normalized utility contract.

## Release Command

- Print one shell-safe two-argument release command after prep in every stack.
- Emit the validated release tag bare in the printed release command.
- Emit the release message single-quoted with POSIX quote escaping in the printed release command.
- Confine the tag to `v`, digits, and dots through each adapter's existing validation before emission.
- Route each adapter's release-command prints through one emit helper.
- Reject a Rust, Terraform, or Swift release whose tag, message, or CHANGELOG row prep did not prepare in the current repository before the approval prompt.
- Accept `--yes` or `-y` after the two release arguments in every stack.
- Print the same candidate files and release sequence under `--yes` as the interactive release.
- Skip only the approval prompt under `--yes`.
- Keep `--yes` out of the printed release command.
- Refuse a release from a detached `HEAD` before the approval prompt in every stack.
- Push the branch with `git push -u origin HEAD` when it has no upstream.
- Keep each stack's branch push when the branch has an upstream.

## Child Repositories

- Read `govna/children.txt` at the start of every build in every stack.
- Reject a registry without the `govna-children-v1` header, a final newline, or unique, byte-sorted, slash-normalized relative paths.
- Run each declared child's `build.sh` before the parent's own build steps during a full build.
- Run a child only when every build tooling file matches the parent's copy byte for byte.
- Treat `build.sh` as the build tooling in Go, Swift, and Terraform, and `build.sh` plus `tests/build_cli.sh` in Rust.
- Stop the parent build when a child is missing, sits inside the parent, contains the parent, has different build tooling, or fails its build.
- Name the child and its recovery action in every child-build failure.
- Skip child builds during a scoped build and print one line saying so.
- Keep each child build's own side effects, such as installing a Go child's utilities.
- Stop prep and release when a local override resolves to a declared child.
- Name the child, the override, and the pin update that removes it in that message.
- Accept a local override to a declared child during a full build.
- Accept every local override that resolves outside every declared child.
- Clone each declared child at its registry path before a full build, including in CI.
- Run a scoped build in a clone that lacks its children.

## Markdown Identifier Check

- Scan every tracked or unignored Markdown file for bare AC, AT, and IE identifiers.
- Run the scan before the first formatting step of every full and scoped build.
- Run the scan at the start of every prep, before any write or dry-run report.
- Fail on each identifier that `AGENTS.md` `### AC Mechanics` and `### Child Repositories` do not allow.
- Treat an uppercase AC, AT, or IE followed by digits, with no adjacent letter, digit, or underscore, as bare.
- Treat a one-line `<!-- Historical: ... -->` comment as a Markdown `Historical:` comment.
- Treat a repository as a child when its `AGENTS.md` has a line that starts with `This repository is a Govna child repository.`.
- Skip `tests/fixtures/` and every `testdata/` directory.
- Name the file, line, identifier, and a placeholder fix such as `AC<N>` in each finding.
- Keep the scan in one helper block that is identical in every build script.

## Go

- Discover utilities from regular `cmd/<target>/main.go` files in byte order.
- Validate scoped target names against discovered utilities before running mutable build steps.
- Reject unknown, duplicate, and path-containing scoped targets.
- Compile each selected utility once in an invocation-owned external temporary directory.
- Validate every selected compiled `--version` output before installing any selected utility.
- Validate every selected compiled utility's help output against `## CLI Usage Formatting` before installing any selected utility.
- Require `-h`, `-?`, and `--help` to print identical help on stdout with exit 0 and no escape sequence.
- Require help line three to be the `go.mod` module path, optionally followed by `/` plus a sub-path.
- Require a utility README at `cmd/<name>/README.md`, when present, to carry a `### Usage` text block byte-equal to the help.
- Replace each installed utility atomically from an adjacent staging file.
- Preserve each installed utility until its replacement succeeds.
- Require every prep-changed version declaration to equal the unprefixed release tag.
- Accept each `\|` pair in an existing summary cell as one escaped pipe during changelog shape validation.
- Reject every release-message pipe, escaped or raw.
- Emit no validation token from Go builds.
- Remove invocation-owned build, coverage, version-probe, and installation-staging outputs on every handled exit.
- Terminate after handling HUP, INT, or TERM.
- Infer Go from `go.mod`.
- Select Go explicitly with `--stack Go`.
- Require the Go toolchain and the pinned staticcheck version installed by `build.sh`.
- Pin `GOTOOLCHAIN` in `build.sh` to the `go.mod` `toolchain` directive, or else to its `go` directive, before the first Go command.
- Name the pinned staticcheck, the active `GOTOOLCHAIN`, and both remedies when staticcheck cannot decode the toolchain's export data.
- Run dependency tidying, formatting, fixes, vetting, tests with coverage, staticcheck, and compilation.
- Install command binaries into `$(go env GOPATH)/bin`.
- Bump the single detected `programVersion` during release prep.
- Derive the primary utility name from the `go.mod` module path without its major-version suffix.
- Reject a release tag at v2 or later when the `go.mod` module path lacks the matching `/v<major>` suffix.
- Accept a repository with no commits during release prep.
- Capture the candidate Git tree during a release-prep dry run.
- Accept command names for scoped builds while retaining package-wide shared validation.
- Capture the complete candidate Git tree without changing the repository index.
- Create the first commit during release in a repository with no commits.
- Display the candidate files and exact release sequence before approval.
- Reject a changed candidate tree after approval.
- Verify the staged and committed trees against the approved candidate.
- Require the release commit to use the exact approved message.
- Require clean committed `HEAD` before release compilation.
- Reuse an approved retry commit only when the tag is absent, Git state is clean, the full commit message matches, and prepared metadata matches.
- Compile each discovered utility once in byte order with `go build -mod=readonly -buildvcs=true -o <temporary-output> -ldflags '-s -w' ./cmd/<target>`.
- Run no canonical validation phase during release compilation.
- Validate every compiled utility version and committed-HEAD provenance before installation.
- Validate every compiled utility's help output before installation during release compilation.
- Preserve independent secondary utility versions during release compilation.
- Install every validated utility by atomic adjacent replacement.
- Recheck every installed utility version and committed-HEAD provenance before tagging.
- Recheck every installed utility's help output before tagging.
- Create the release tag only after compilation, validation, installation, and rechecks pass.
- Push nothing after an earlier release failure.
- Remove invocation-owned release outputs on every handled exit.
- Release a repository with no `cmd/<target>/main.go` from its tag alone, without compiling or installing a utility.
- Use `go work init . <child>` for a child override.
- Set the `go.work` `go` line to the `go.mod` `go` line with `go work edit -go=<version>`.
- Detect a child override in a `go.mod` `replace` directive and in the workspace that `go env GOWORK` names.
- Skip `go mod tidy` in a full build only while that workspace names a declared child.
- Print one line that names the child when the build skips `go mod tidy`.
- Move a pin with `go mod edit -dropreplace=<module>` when a replace exists, then `go get <module>@<tag>`.
- Set `GOPRIVATE` or `GOPROXY=direct` when `go get` cannot see a just-pushed tag.

## Rust

- Infer Rust from `Cargo.toml`.
- Select Rust explicitly with `--stack Rust`.
- Require Cargo, rustfmt, and Clippy.
- Run formatting, Clippy, tests, and release compilation.
- Keep compilation in an invocation-owned external Cargo target.
- Install binaries into `$CARGO_HOME/bin`, or `$HOME/.cargo/bin` when `CARGO_HOME` is unset.
- Bump the root package version during release prep.
- Refresh `Cargo.lock` during release prep.
- Accept declared binary names for scoped builds.
- Preserve package-wide shared validation during scoped builds.
- Require one literal `PROGRAM_VERSION: &str` strict stable SemVer declaration in each declared binary path.
- Use a `[patch]` entry with a `path` for a child override.
- Detect a child override in a `path` dependency or `[patch]` entry in `Cargo.toml` or `.cargo/config.toml`.
- Move a pin by setting the dependency's `tag`, removing the override, then running `cargo update -p <crate>`.

## Terraform

- Infer Terraform from `.terraform.lock.hcl` or root Terraform files.
- Select Terraform explicitly with `--stack Terraform`.
- Require the Terraform CLI.
- Run recursive formatting checks and module validation.
- Keep Terraform working data in repository-local ignored artifact directories.
- Derive release versions from Git tags without a source version bump.
- Reject scoped builds because Terraform validation is repository-wide.
- Use a local module `source` for a child override.
- Detect a child override in any `.tf` module `source`.
- Move a pin by setting `?ref=<tag>` in each child module `source`.

## Swift

- Infer Swift from a root `Package.swift`.
- Select Swift explicitly with `--stack Swift`.
- Prefer Go, Terraform, and Rust manifests over Swift.
- Require Swift 6.0 or newer, Git, and one root SwiftPM package on macOS or Linux.
- Run strict toolchain formatting, debug compilation, tests, and release compilation with compiler warnings as errors.
- Keep SwiftPM artifacts in one invocation-owned external scratch directory and clean it on success, failure, and handled signals.
- Keep project-level `.swiftpm/` configuration trackable.
- Keep `Package.resolved` tracked for leaf packages with dependencies.
- Treat `Package.resolved` as optional for dependency libraries.
- Derive release versions from Git tags.
- Leave `Package.swift` unchanged during release prep.
- Install executable products into `${SWIFT_BIN_HOME:-$HOME/.local/bin}` by atomically replacing regular destination files and refusing unsafe entries.
- Let library-only packages complete without installation.
- Use `swift package edit --path <child> <package>` for a child override.
- Detect a child override in `.package(path:)` and in edit mode.
- Move a pin by setting the version in `Package.swift`, running `swift package unedit <package>`, then `swift package resolve`.
- Use the canonical color and plain-text presentation policy for build, prep, and release output.
- Accept executable-product names for scoped builds while retaining package-wide formatting and tests.
- Build only selected executable products during scoped builds.
- Install only selected executable products during scoped builds.
- Treat native Xcode projects and Apple application bundles as a possible future backend.
