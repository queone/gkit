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
- `oidctok`: GitHub Actions job helper that fetches the job's OIDC token and exchanges it at the Microsoft identity platform for Resource Manager and Graph tokens over HTTPS with the standard library only; it keeps no local state and writes tokens only to the `GITHUB_ENV` file

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
- `help`, `color`, and `lockbox` are public packages: `help` and `color` serve gkit's utilities, and all three stay public because tools in other repos import them; `lockbox` has no importer inside gkit. `help` renders every utility's help screen from a `help.Doc`: the three-line header (name and version, description, repository URL), capitalized bold-white section headings, aligned rows, and the two standard `Options` rows, so no utility colors or aligns help by hand. `lockbox` names no tool: each caller sets its own Keychain service, and every store file keeps the `MACFIT` format tag. `internal/numfmt` and `internal/vedit` stay private to gkit: `numfmt` is the single thousands-separator implementation, and `vedit` is the shared ffmpeg/ffprobe driver behind `vkeep`, `vdrop`, `vconv`, and `vshrink`.

## Conventions

- update this document when architecture or major workflow changes materially
- keep implementation detail in code and stable architecture here
