# gkit
A collection of small CLI utilities written in Go.

## Project Direction

gkit is an active collection of Go utilities. `repoctl` consolidates the former Git helper commands into one maintained interface, while the remaining utilities retain their existing names and behavior.

## Why
Go's tool chain is the ideal way to maintain a set of commonly used CLI utilities. They can be quickly compiled and installed whether you're in Windows, macOS, or Linux. This provides a unified and portable solution to many a scripting needs. With this setup, Go turns into a quasi-package manager for these utilities.

## Utilities

- [`bak`](cmd/bak/main.go): Create dated backups of files or directories.
- [`brew-update`](cmd/brew-update/main.go): Update, upgrade, and clean up Homebrew packages.
- [`cash5`](cmd/cash5/main.go): Analyze historical NJ Cash 5 draws (1-45 era, starting 2014-09-14) and generate number recommendations guaranteed to be unwon combinations.
- [`certgen`](cmd/certgen/main.go): Generate self-signed TLS certificates for local testing.
- [`certls`](cmd/certls/main.go): Show SSL/TLS certificate details for a host and port.
- [`days`](cmd/days/README.md): A CLI calendar days calculator.
- [`decolor`](cmd/decolor/README.md): A utility that removes shell color escape codes from input stream or given file.
- [`dl`](cmd/dl/main.go): Download online videos using `yt-dlp` with a target filename.
- [`dos2unix`](cmd/dos2unix/main.go): Preview or convert CRLF line endings to LF.
- [`fr`](cmd/fr/README.md): A simple find/replace utility.
- [`repoctl`](cmd/repoctl/README.md): Control collections of local Git repositories with status, pull, build, clone, and list operations.
- [`jy`](cmd/jy/README.md): A lightweight JSON and YAML converter utility.
- [`mdview`](cmd/mdview/README.md): Render GitHub Flavored Markdown in a browser or write it as HTML.
- [`pgen`](cmd/pgen/README.md): A simple generator of memorable passwords.
- [`pman`](cmd/pman/main.go): Run authenticated Microsoft Graph and Azure REST API requests.
- [`retotal`](cmd/retotal/README.md): Recalculate TOTALS in a signed financial summary; also consolidates CSV/aligned input into a signed summary.
- [`rn`](cmd/rn/README.md): A bulk file re-namer.
- [`rncap`](cmd/rncap/main.go): Rename files by capitalizing each word in filenames.
- [`rnlower`](cmd/rnlower/main.go): Rename files by converting filenames to lowercase.
- [`sms`](cmd/sms/README.md): Send SMS messages using Twilio credentials from a local config file.
- [`tree`](cmd/tree/README.md): A lightweight directory tree printing utility.
- [`vdrop`](cmd/vdrop/README.md): Remove a section of a video — drop START..END and join the remainder — via ffmpeg.
- [`vjoin`](cmd/vjoin/README.md): Join two videos with orientation-aware framing and normalized output via ffmpeg.
- [`vkeep`](cmd/vkeep/README.md): Keep a section of a video — extract START..END to a new file — via ffmpeg.
- [`web`](cmd/web/README.md): Search DuckDuckGo and open results with an interactive selector.

## Quick Install
With Go installed, install all utilities at once:

```bash
go install github.com/queone/gkit/cmd/...@latest
```

Or install a single utility:

```bash
go install github.com/queone/gkit/cmd/fr@latest
```

Binaries are placed in `$GOPATH/bin` (typically `~/go/bin`), which should be in your `$PATH`.

## Getting Started
To compile the entire collection, you obviously need to have GoLang installed and properly setup in your system, with `$GOPATH` set up correctly (typically at `$HOME/go`). Also setup `$GOPATH/bin/` in your `$PATH`, since that is where all executable binaries will be placed.

To compile for the first time do: 

```bash
git clone https://github.com/queone/gkit
cd gkit
go mod init gkit
go mod tidy
./build.sh
```

For subsequent compilation just: 

```bash
cd gkit
git pull
./build.sh
```

Note that you can compile individual utilities with `./build.sh rn web`, etc. Targets are space-separated; validation runs only against the named packages.

To build in Windows you have to have a BASH shell such as [GitBASH](https://www.git-scm.com/download/win). To build from a regular Windows Command Prompt, you may have to tweak the `build.sh` script a bit, to have it run the right `go build ...` command.

## Governance

This repo is governed by an explicit session-entry contract for AI coding agents — see [`govna/operator-contract-rationale.md`](govna/operator-contract-rationale.md) for the design reasoning and [`AGENTS.md`](AGENTS.md) for the operational rules.
