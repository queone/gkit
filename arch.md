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

## Conventions

- update this document when architecture or major workflow changes materially
- keep implementation detail in code and stable architecture here
