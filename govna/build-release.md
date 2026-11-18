# Build and Release

## Build and Test Rules

- Keep one documented canonical build command.
- Route formatting, checks, tests, and packaging through it.
- Keep release work out of routine implementation.

Use self-contained `build.sh` for build, release prep, and release work without external govna tools.

## Minimum Validation

- Require formatting, static checks, automated tests, and behavior-aligned docs to pass.



## Canonical Build Commands

```bash
./build.sh
```

To scope the run to selected commands:

```bash
./build.sh <target> [<target> ...]
```

Use space-separated target names. Supported CODE stacks may retain package-wide shared-code validation while limiting target-specific checks, tests, artifacts, and installation to the selected targets.

Run `./build.sh` without targets for repository-wide validation. Follow the applicable stack guidance above for release-prep evidence, pre-change validation, and build-state reuse. Release-prep validation uses the package-wide form.

## Independent Utility Versions

- Treat the repository/package version as the version input and release metadata governed by the existing release mechanism.
- Require one normalized record for each installable utility with its canonical target name, declaration location, declared version, and `--version` invocation.
- Accept only `^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$` as a strict stable SemVer declaration.
- Require `--version` to exit 0.
- Require `--version` to print exactly `<utility-id> v<MAJOR.MINOR.PATCH>` plus its newline to stdout.
- Require `--version` to write nothing to stderr.
- Validate every declaration before compilation.
- Validate each compiled utility result before installing that utility.
- Validate every compiled utility result before release-metadata writes.
- Reject missing, empty, malformed, duplicate, orphaned, and mis-mapped records with a non-zero error that names the utility and recovery action.
- Preserve all independent utility declarations and outputs during repository release prep.

## Pre-Release Checklist

- Apply this checklist only to an explicit Director Package instruction for an established Ratified release batch or direct batch.

Note: the operator flow has two steps. Prep stops on a bare AC, AT, or IE identifier that `AGENTS.md` `### AC Mechanics` does not allow.

1. **Run prep.**
   - Classify the AC scope under semver.
   - Draft a release message that names the delivered user-visible result and every established release-batch AC reference in no more than 80 bytes.
   - Name each direct-handled companion change inside that user-visible result without an AC reference.
   - Keep the release message on one line and inside one Markdown table cell.
   - Exclude every AC reference outside the established release batch.
   - Rerun applicable validation before prep when Package evidence is missing or stale.
   - Run the stack-defined `./build.sh prep vX.Y.Z "message"` invocation.
   - Keep Go prep free of canonical build, Go build, and Go dependency commands.
   - Use `--dry-run` or `-n` to inspect without writes.

   Before running prep, satisfy this repository's declared version-target contract and keep repository/package and independently versioned utility declarations aligned as required by its Project Practices.
2. **Run the printed release command.**
   - Run `./build.sh vX.Y.Z "message"`.
   - Confirm the displayed candidate files and exact release sequence.
   - Approve the interactive prompt to commit, compile, validate, install, tag, and push.
   - Append `--yes` or `-y` to skip that prompt only for an agent-run release under `AGENTS.md` `### Agent Release`.

- End the structured Package completion report with `Run below to release:`.
- Place the exact release command immediately after that line.
- Add nothing after the release command.

## CHANGELOG Conventions

- Use a `# Changelog` heading.
- Follow it with the two-column `| Version | Summary |` table.
- Use `|---------|---------|` as the separator.
- Keep `| Unreleased | |` as the first data row.
- Add one row per release.
- Keep summaries single-line and no longer than 500 characters.
- Escape each literal pipe in a summary as `\|`.
- Lead summaries with the AC reference when one exists.
- Versions are unprefixed (`0.29.0`, not `v0.29.0`).
- Do not backfill historical tags or invent alternative shapes (Keep-a-Changelog, sectioned `## vX.Y.Z`, etc.).

## Release History Format

- Apply this format to every `history.md` section that Package writes.
- Start a new `history.md` with the heading `# <repository name> History` and the paragraph below.
- Head each release section `## vMAJOR.MINOR.PATCH`, newest release first.
- Order the labels `Added:`, `Changed:`, `Removed:`, `Deferred:`, `Decisions:`, `Risks:`.
- Write each label on its own line, followed by its bullets.
- Omit an empty label.
- Write condensed Simplified Technical English.
- Keep one fact per bullet.
- Keep each bullet to at most 25 words.
- Keep each line of a release section to about 120 characters or fewer.
- Break a longer bullet at a clause boundary within the limit, such as after a comma, semicolon, or colon.
- Break a longer bullet at the last space within the limit when no clause boundary fits.
- Keep each path, URL, identifier, command, and exact quoted output unbroken and unchanged.
- Let a line pass the limit only when a path, URL, identifier, command, or exact quoted output cannot fit within it.
- Indent each later line of a bullet by two spaces.
- Start at the first release after adoption.
- Skip back-filling earlier releases.

> One section per release, newest first: what the release added, changed, removed, or deferred in the product, its design, and its code, and why. Governance, toolchain, and release mechanics stay in `CHANGELOG.md` and git. Settled design is in `arch.md`. Search it to learn why a release changed the product.

## Project Practices
