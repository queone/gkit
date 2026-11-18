Copy this file to `govna/ac<N>-<slug>.md`.
Use a kebab-case slug and a `# AC<N> Title` heading that names the concrete outcome.

Set `N` to one above the highest AC number in `govna/` or `git log --all --pretty=%B`.
Count every reference because release-prep deletions do not reset numbering.

An AC records settled intent, scope, and proof once so later review, implementation, and verification can reuse the same context instead of reconstructing it. `govna/development-cycle.md` describes the cycle; `AGENTS.md` governs drafting, review, and integration.

Size each section to the change: a small fix needs a few lines, not every prompt below.
State each requirement once, in the section that owns it.
Replace the wording a change makes obsolete in every section that carries it.
Keep a reason only where it explains a constraint, a rejected alternative, or a non-obvious test.
Leave discussion history out.
Replace each section's instruction lines with its content.

# AC<N> Title

## Summary

Lead with the concrete outcome in one short paragraph.
Explain each necessary Govna label before relying on it.
Name each Part when the AC has more than one.

## In Scope

List each path with what changes in it, grouped only when the list is long.
Treat this list as authoritative.
Name any schema change with its new version and migration step.

## Out Of Scope

List only adjacent work a reader could mistake for scope, or write `None.`

## Migration findings

List each audit `migration-required` item with the consumer action that completes it, and keep it under `## In Scope` too.
Delete this section when audit emitted none.

## Acceptance Tests

Write one test per pass/fail behavior, each with its exact check: a command, file state, pattern, or output.
Label each test `[Automated]` or `[Manual]` and `[Pre-release gate]` or `[Post-release verification]`.
Prefer automated pre-release tests; use post-release only when automated regression coverage already gates the behavior class.

**AT1** [Automated] [Pre-release gate] — The behavior verified, with its exact check.

**AT2** [Manual] [Pre-release gate] — The behavior verified, with the live action that confirms it.

## Status

`PENDING` — awaiting user authorization to begin implementation.

Use `IN PROGRESS`, or `DEFERRED` with a reason, and list status by phase for partial completion. Never write `DONE`: release prep deletes the AC.
