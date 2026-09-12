## tfe
Terraform Cloud command-line utility: list organizations, registry modules, and workspaces, show a workspace with its variables, and clone a workspace.

### Why?
The Terraform Cloud web interface is fine for one workspace at a time and cannot clone one at all. `tfe` answers the everyday questions from the shell, with a substring filter on every list, and copies a workspace's settings and variables into a new one in a single command.

```bash
tfe ws prod
prod-network
prod-database

tfe show prod-network
# Terraform Cloud Workspace
workspace_name: prod-network
workspace_id: ws-abc123
created_at: 2026-Sep-08 09:30
...
```

### Usage

```bash
tfe [flags] SUBCOMMAND [ARGS]
```

| Subcommand | What it does |
|---|---|
| `orgs [FILTER]` | List organizations |
| `mods [-a] [-j] [FILTER]` | List registry modules, latest version each; `-a, --all` lists every version; a single match prints its details, or JSON with `-j, --json` |
| `ws [FILTER]` | List workspaces |
| `show NAME` | Show one workspace, its agent pool when it runs in agent mode, and its variables by category |
| `clone SRC DEST` | Create `DEST` with `SRC`'s settings and copy every variable |

`FILTER` is a case-insensitive substring of the name. Module addresses are printed under the host of your `TF_DOMAIN`.

### Authentication
`tfe` reads three values, from the environment first:

```bash
export TF_ORG=myorg
export TF_DOMAIN=https://app.terraform.io
export TF_TOKEN=...
```

When none of the three is set it reads `$XDG_CONFIG_HOME/tfe/config.yaml`, or `~/.config/tfe/config.yaml`:

```yaml
TF_ORG:     myorg
TF_DOMAIN:  https://app.terraform.io
TF_TOKEN:   ...
```

A missing or empty config file is created as a skeleton with mode 0600 so you can fill it in. The token is never printed.

### Cloning and sensitive variables
The API never returns the value of a sensitive variable, so `clone` creates those variables in the destination with empty values and prints their names. Set them again in the new workspace before running it.

### History
This utility replaces the standalone `queone/tfe` repository, which depended on the `queone/utl` library. Both have been deleted; this is where the tool lives now.

### Getting Started
This utility is part of a collection of Go utilities. To compile and install follow the **Getting Started** instructions at the [gkit repo](https://github.com/queone/gkit).
