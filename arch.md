# gkit Architecture

## Purpose

A collection of small CLI utilities written in Go.

## System Summary

Document the system's major components, boundaries, runtime flow, storage model, and external integrations here.

## Current Platform

- Go

## Major Components

- entrypoints and user-facing surfaces
- core domain or business logic
- storage, messaging, or state boundaries
- external integrations and trust boundaries
- `repoctl`: consolidated local Git repository management through `git` and scoped GitHub operations through `gh`
- `attune`: declarative reconciler comparing YAML specs against live Azure Resource Manager and Microsoft Graph state (DNS, security groups, app registrations, roles, resource groups); the repo's first utility with live, state-mutating external-integration calls

## Core Files

- `AGENTS.md`: base governance contract
- `plan.md`: prioritized roadmap and approved direction
- `build.sh`: self-contained build, release-prep, and release tooling
- `governa/development-cycle.md`: workflow from roadmap through release
- `governa/ac-template.md`: acceptance-criteria template for new work
- `governa/build-release.md`: build, test, and release rules

## Data And Control Flow

Describe the main request, job, or publish path from entrypoint to output.

## Architecture Notes

- record stable system decisions here
- prefer durable structure and interfaces over transient implementation detail
- `cash5` operates on the 1-45 era only. Draws with `DrawTime` before `cash5EraStartMillis` (2014-09-14 UTC, the first 1-45 pool draw) are pruned at load and the local `draws.json` is rewritten in place; pre-cutoff history is not retained. Recommendation generation enforces a uniqueness invariant against the post-cutoff winners set.
- `attune` authenticates via the local `az` CLI (`az account show`/`az account get-access-token`), never its own credential store; it writes no local state, cache, or telemetry — the live provider is the only source of truth. Requests are restricted to an ARM/Graph origin allowlist, and non-2xx response bodies are redacted wholesale before reaching any diagnostic or error message, so a failure can never leak provider response content. Role-assignment/definition resource IDs are derived deterministically (SHA1-based, RFC4122 URL-namespace UUIDv5) to stay stable across repeated `apply` runs and compatible with resources rkit's `attune` already created live.

## Conventions

- update this document when architecture or major workflow changes materially
- keep implementation detail in code and stable architecture here
