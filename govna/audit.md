# Audit

Run `govna audit` without positional arguments from the root of a repository that contains Govna files. Audit compares those files with the governance files built into the executable and writes a reviewable AC only when updates or Director choices are needed.

## Usage

```
govna audit [options]
```

Flags:

Here, flavor means the CODE or DOC set of Govna files selected for the repository.

- `-f, --flavor code|doc` — overlay flavor (default: auto-detect from repo signals).
- `-s, --stack <name>` — CODE stack (default: inferred from manifests; not accepted with `--flavor doc`).
- `-c, --check` — report the result without writing an AC; exit `3` when updates or Director choices are needed.
- `-u, --with-utilities` — compare against the files for a CODE repository with an installable utility, before the first utility exists.
- `-j, --json` — also print a JSON report to stdout alongside the markdown emission.
- `-l, --diff-lines <N>` — diff truncation limit (default: 200).
- `-n, --repo-name <name>` — override repo name (default: basename of the target directory).
- `-h, --help` — show this help.

Require `AGENTS.md`, a Git worktree, and evidence that Govna was added: a Govna AC, release, or build-release file, or a CHANGELOG reference to `govna apply` or `govna render`.

## Classification

The classification is the exact result label beside each file.

Every emitted audit AC uses these shared explanations:

| Classification | Explanation |
|---|---|
| `match` | The file already needs no Govna update. |
| `missing-in-target` | A file from current Govna rules is missing from the repository. |
| `expected-divergence` | The repository is expected to keep its own version of this file. |
| `preserve` | The preserve list says to keep the repository's version. |
| `clear-sync` | The file still matches the previously installed Govna version and is safe to update. |
| `ambiguity` | Govna cannot safely choose between updating and keeping the file. |
| `target-has-no-canon` | The file is absent from the selected current canon but specific repository evidence connects it to Govna. |
| `migration-required` | A required Govna control file is missing and must be added through the AC. |
| `retired-link` | A Govna link that Claude Code no longer needs and the AC deletes. |
| `retired-file` | A document that left the Govna files, that the repository never edited, and that the AC deletes. |
| `force-sync` | This file's governed structure always syncs to canon, regardless of local edits or the preserve list. |

## Mixed-content boundary registry

Files with a documented canon-above/local-below boundary, compared only above the boundary line for `match`:

| File | Boundary |
|---|---|
| `AGENTS.md` | `## Project Rules` |
| CODE `govna/build-release.md` | `## Project Practices` |
| `govna/development-guidelines.md` | `## Project Practices` |
| `govna/editing-guidelines.md` | `## Project Practices` |

## Preserve registry

Use optional `govna/preserve.txt` as the sole durable preserve authority. Treat an absent file as an empty registry. Require an existing file to use this exact schema:

```text
govna-preserve-v1
<repo-relative-path>
```

Require a final newline. Keep entries nonempty, slash-normalized, unique, and byte-sorted. Reject absolute paths, backslashes, tabs, blank entries, `.` or `..` components, leading or trailing slashes, duplicates, and `govna/preserve.txt` itself. Accept a header-only file as the canonical empty on-disk registry.

Add an exact path for a resolved preserve outcome. Remove an exact path for a resolved sync, delete, or canon-backed migration outcome. Preserve unrelated entries. Leave the registry absent or unchanged when its state already satisfies every resolved outcome. Verify registry changes before installing the canon baseline.

## Repository-check registry

Use optional `govna/repo-check.txt` as the Director's standing repository-check resolution. Treat an absent file as no configuration. Require an existing file to use this exact schema:

```text
govna-repo-check-v1
<command>
```

Require a final newline. Require exactly one non-empty command line after the header. Reject a command containing a backtick. Reject any additional content. Fail the audit with a validation error before any AC emission when the file is malformed.

- Require explicit Director authorization for every `govna/repo-check.txt` creation or modification.
- Treat a valid configured command as the standing Director resolution for the emitted repository check.

## Emitted AC stub

Audit writes `govna/ac<N>-audit-<canon-version>.md` only when the repository needs an update or a Director choice. The stub is immutable: delete or rename an edited stub before running audit again.

### Agent-mediated review

- Follow `AGENTS.md` `### Audit Adoption` for every phase entry, pause, and exit of this review.
- Resolve the Govna executable path before running the agent-mediated audit.
- Record the detailed version output from that resolved executable.
- Run the ordinary agent-mediated audit without `--json`.
- Require the emitted AC marker versions to match the recorded detailed version.
- Create exactly one unique system-temporary scratch directory outside the consumer repository.
- Render the selected canon into that scratch directory once with the resolved executable.
- Require the rendered baseline canon version to match the emitted AC marker.
- Compare every actionable path through the emitted `### Audit Review` instructions.
- Review the exact proposed rules.
- Resolve every candidate reference.
- Verify target-side acceptance evidence.

Note: the executable ends after deterministic comparison and emission. JSON remains available as optional machine output, but its diff fields are not required evidence for ordinary agent-mediated review.

### Routing capabilities

- Offer only these outcomes for a canon-backed ambiguity: sync, preserve, explicitly named migration, delete.
- Offer only these outcomes for an ordinary `target-has-no-canon` item: preserve, explicitly named migration, delete.
- Require the Director to name every migration destination in the routing response.
- Install an exact current-canon replacement before retired-source routing.
- Offer only these outcomes for that retired source: preserve, explicitly named migration, delete.
- Omit restore as a routing outcome.

### Mixed-content sync verification

- Recompute the protected-region digest after adoption.
- Require the protected-region digest to match the emitted digest.
- Keep rendered-canon comparison scoped to the canon zone above the boundary.
- Apply that scope to Audit Review when both files contain the exact registered boundary.
- Keep whole-file Audit Review comparison when either file lacks the exact registered boundary.
- Preserve existing missing-target and target-only Audit Review commands.
- Avoid comparing the repository-owned tail with rendered defaults.

## Canon and local text

- Treat `govna/metadata.txt` as the record of a repository's Govna file version and CODE or DOC type.
- Pause when a canon update introduces an Instruction Style violation.
- Skip local rewrites of canon-owned text unless an explicit AC declares intentional divergence.

## Adoption procedure

- Apply this procedure to each routing action the Director resolves for an emitted adoption AC under `AGENTS.md` `### Audit Adoption`.
- Follow the `### Adoption Instructions` of the emitted AC together with this procedure.
- Render canon into a scratch directory using `govna render <scratch>`.
- Inspect changes per `## In Scope` item by running `diff -ru <scratch>/<path> <path>`.
- Ensure the parent directory exists for each `## In Scope` item: `mkdir -p "$(dirname <path>)"`.
- Categorize each `## In Scope` item as pure-canon or mixed-content before applying.
- Apply pure-canon items by copying from canon: `cp <scratch>/<path> <path>`.
- Apply mixed-content items by hunk-merge.
- Replace canon-zone content above each registered boundary heading.
- Use the boundary registered in `## Mixed-content boundary registry` for each mixed-content item.
- Put exactly one blank line between each pair of adjacent IE items in a `plan.md` listed under `### IE items to space`.
- Keep every other `plan.md` line, each IE item's text, and their order.
- Run the chosen repository command after all selected sync, migration, and deletion work.
- Verify each resolved sync target against its applicable rendered canon region.
- Verify each migration source is absent unless the Director explicitly preserves it.
- Verify each canon-backed migration destination against its applicable rendered canon region.
- Verify each repo-owned migration destination against the Director's stated result.
- Verify each resolved delete target is absent.
- Delete the `CLAUDE.md` link for a `retired-link` item.
- Verify the retired `CLAUDE.md` link is absent.
- Delete each file listed under `### Retired Govna files` of the emitted AC.
- Verify each of those files is absent.
- Remove or repoint each reference listed under `### References to retired files` of the emitted AC.
- Verify each resolved preserve target remains and its exact path occurs in `govna/preserve.txt`.
