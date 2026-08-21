# Development Guidelines

Use these durable coding practices.
Use `AGENTS.md`, `development-cycle.md`, and `build-release.md` for workflow, validation, and Package.
Sections above ## Project Practices are govna-maintained canon and update via canon syncs; repo-specific practices in ## Project Practices.

## Identifier Strategy

- Choose a primary key strategy early and document it in `arch.md`
- Prefer surrogate keys for internal identity
- Keep external IDs as indexed attributes
- When integrating multiple external ID systems, maintain an explicit mapping layer rather than assuming IDs are interchangeable

## Schema And Data Migrations

- Treat schema changes as first-class events: version them, document them, test the migration path
- Never assume old data fits new schemas — write migration logic or fail explicitly
- When a migration changes identity or key structure, audit all foreign key references in the same change

## External Integration Patterns

- Validate external data at the boundary
- Treat upstream shape and completeness as untrusted
- When reconciling data from multiple sources, define a clear precedence order and document it
- Cache external data locally with explicit TTL or versioning
- Never silently serve stale data as fresh

## Generated Artifact Propagation

- When source-of-truth code is duplicated into templates or rendered examples, fixes must propagate to all copies in the same change
- Grep the full repo for the pattern being changed before considering a fix complete
- If a template and its rendered output diverge, the template is authoritative
- Keep `build.sh` self-contained.
- Do not add sourced production helper modules.

## Error Handling And Validation

- Validate at system boundaries (user input, external APIs, file I/O)
- Trust internal code
- Fail explicitly rather than silently degrading — a clear error is better than wrong output
- Static analysis and linting errors are build failures, not warnings
- Validate installable-target declarations before compiling or installing them.
- Follow the applicable stack guidance for release-prep evidence, validation ordering, and build-state reuse.
- Pass release-prep evidence through the applicable stack's canonical CLI option.

## Testing Expectations

- Test every new function and error path in the implementation pass.
- If a code path cannot be tested without mocking infrastructure that is out of scope, document the coverage gap explicitly rather than silently skipping it
- Label tests that require live systems or manual verification as `[Manual]`

## Dependency And Import Hygiene

- Prefer standard library over external dependencies when the capability is equivalent
- When adding a dependency, justify it — convenience alone is not sufficient
- Keep import paths consistent after renames or reorganizations

## CLI Usage Formatting

- All commands must accept `-h`, `-?`, and `--help` as help flags
- Help output uses a shared formatting function for consistent layout
- "Usage:" is rendered in bold white
- Indent each flag line by 2 spaces
- Align descriptions at column 38
- Short and long flag forms are combined on one line (e.g. `-v, --verbose`)
- When adding new flags, add the entry to the shared usage formatter — do not rely on framework defaults

## Documentation Alignment

- Ship behavior docs with code.
- Verify every referenced symbol or path.
- Keep `arch.md` limited to built architecture.

## Go Practices

- Add single-line godoc comments to exported functions in shared Go packages.
- Declare a non-empty `const programVersion` string literal in every installable `cmd/<name>/main.go`.
- Validate every `programVersion` declaration through `build.sh` before compiling installable binaries.
- Pin `staticcheck` to the repository-governed version and invoke the pinned installation path directly.
- Treat `go vet` and `staticcheck` findings as build failures.
- Scan all `.go` and `.go.tmpl` files for stale import paths after a module rename.

## Project Practices

- Follow existing repo patterns unless an approved improvement says otherwise.
