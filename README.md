# gkit
A collection of small CLI utilities written in Go.

## Project Direction

gkit is an active collection of Go utilities. `repoctl` consolidates the former Git helper commands into one maintained interface, while the remaining utilities retain their existing names and behavior.

## Why
Go's tool chain is the ideal way to maintain a set of commonly used CLI utilities. They can be quickly compiled and installed whether you're in Windows, macOS, or Linux. This provides a unified and portable solution to many a scripting needs. With this setup, Go turns into a quasi-package manager for these utilities.

## Utilities

- [`attune`](cmd/attune/README.md): Reconcile Azure state from YAML specs kept in an encrypted store.
- [`bak`](cmd/bak/main.go): Create a dated backup copy of a file or directory.
- [`brew-update`](cmd/brew-update/main.go): Update, upgrade, and clean up Homebrew formulae and casks.
- [`cash5`](cmd/cash5/main.go): Recommend NJ Cash 5 numbers from the draw history.
- [`certgen`](cmd/certgen/main.go): Generate a self-signed TLS certificate, private key, and CSR for a common name.
- [`certls`](cmd/certls/main.go): Print the TLS certificate details of a host and port.
- [`days`](cmd/days/README.md): Count calendar days between dates, or find the date N days away.
- [`decolor`](cmd/decolor/README.md): Strip shell color escape codes from a file or piped text.
- [`dl`](cmd/dl/main.go): Download an online video as an MP4 with yt-dlp.
- [`dos2unix`](cmd/dos2unix/main.go): Preview or convert CRLF line endings to LF.
- [`fr`](cmd/fr/README.md): Find a regular expression in the text files under the current directory, or replace it.
- [`ishrink`](cmd/ishrink/README.md): Shrink a HEIC, JPEG, or JPG image into a small JPEG via macOS sips.
- [`repoctl`](cmd/repoctl/README.md): Control a collection of local Git repositories.
- [`jy`](cmd/jy/README.md): Convert JSON to YAML and YAML to JSON.
- [`macfit`](cmd/macfit/README.md): Keep Mac config files in one encrypted store and restore them on any Mac.
- [`mdview`](cmd/mdview/README.md): View GitHub Flavored Markdown in a browser or write it as HTML.
- [`namehunt`](cmd/namehunt/README.md): Find free usernames on any site with a predictable profile URL.
- [`oidctok`](cmd/oidctok/README.md): Exchange a GitHub Actions OIDC token for Azure tokens.
- [`pgen`](cmd/pgen/README.md): Generate memorable passwords from diceware words.
- [`pman`](cmd/pman/main.go): Call Microsoft Graph and Azure Resource Manager REST APIs with an azm token.
- [`retotal`](cmd/retotal/README.md): Consolidate financial data into a signed TOTALS summary, and re-tally it after edits.
- [`rn`](cmd/rn/README.md): Rename files in the current directory by replacing a substring.
- [`rncap`](cmd/rncap/main.go): Capitalize every word of every file name in the current directory.
- [`rnlower`](cmd/rnlower/main.go): Rename every file in the current directory to lowercase.
- [`sms`](cmd/sms/README.md): Send an SMS message through textbelt.com.
- [`swatch`](cmd/swatch/README.md): Inspect the xterm 256-color palette and the color ramps.
- [`tfe`](cmd/tfe/README.md): List, show, and clone Terraform Cloud workspaces, modules, and organizations.
- [`tree`](cmd/tree/README.md): Print a directory tree, with each file's full path on request.
- [`vconv`](cmd/vconv/README.md): Convert a video to an H.264 MP4 by driving ffmpeg.
- [`vdrop`](cmd/vdrop/README.md): Drop one section of a video and join the rest by driving ffmpeg.
- [`vjoin`](cmd/vjoin/README.md): Join two videos into one normalized MP4 by driving ffmpeg.
- [`vkeep`](cmd/vkeep/README.md): Keep one section of a video by driving ffmpeg.
- [`vshrink`](cmd/vshrink/README.md): Shrink an MP4 by re-encoding it at a high compression level via ffmpeg.
- [`web`](cmd/web/README.md): Search DuckDuckGo and open a result picked with a fuzzy finder.

## Quick Install
With Go installed, install all utilities at once:

```bash
go install github.com/queone/gkit/cmd/...@latest
```

Or install a single utility:

```bash
go install github.com/queone/gkit/cmd/fr@latest
```

Binaries are placed in `$GOPATH/bin` (typically `~/.go/bin`), which should be in your `$PATH`.

## Getting Started
To compile the entire collection, you obviously need to have GoLang installed and properly setup in your system, with `$GOPATH` set up correctly (typically at `$HOME/.go`). Also setup `$GOPATH/bin/` in your `$PATH`, since that is where all executable binaries will be placed.

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

## Scripts

Standalone scripts that are fetched rather than installed. Download any of them with `curl -L` from `https://github.com/queone/gkit/raw/main/scripts/<file>`, which redirects to the raw host.

- `aztoken.py`: Azure token demo, run under Docker Compose.
- `aztoken_compose.yaml`: Docker Compose file for the Azure token demo.
- `bashrc_user.sh`: Generic interactive bash settings for a user account on macOS, installed by copying.
- `bashrc_root.sh`: Generic interactive bash settings for the root account on macOS, installed by copying.
- `gitbranch.sh`: Fast git branch indicator for the bash prompt; `bashrc_user.sh` sources it from `~/.config/bash/gitbranch.sh`.
- `install_docker.sh`: Install or upgrade Docker, through the docker CLI and colima on a Mac, Docker's apt repository on Debian or Ubuntu, and Docker's dnf repository on Fedora and RHEL-family systems.
- `install_go.sh`: Install or upgrade Go, through Homebrew on a Mac when present and the official tarball otherwise.
- `install_tf.sh`: Install or upgrade Terraform, through Homebrew on a Mac, HashiCorp's apt repository on Debian or Ubuntu, and the official archive otherwise.
- `install_vault.sh`: Install or upgrade HashiCorp Vault, through Homebrew on a Mac, HashiCorp's apt repository on Debian or Ubuntu, and the official archive otherwise.
- `mac_screencap.sh`: Adjust the macOS Shift-Cmd-4 screen-capture settings.
- `get_oidc_tokens.py`: Python option to exchange a GitHub Actions OIDC token for Azure tokens; the `oidctok` utility is recommended for simplicity and speed.
- `dns_chk.ps1`: Verify Active Directory A and PTR records from an input file.
- `dns_add.ps1`: Create Active Directory DNS records from an input file.
- `dns_del.ps1`: Delete Active Directory DNS records from an input file.
- `dns_upsert.ps1`: Create or update Active Directory DNS records from an input file.

## Governance

This repo is governed by an explicit session-entry contract for AI coding agents — see [`govna/operator-contract-rationale.md`](govna/operator-contract-rationale.md) for the design reasoning and [`AGENTS.md`](AGENTS.md) for the operational rules.
