# attune
A small declarative reconciler: describe provider-neutral intent in YAML and compare it directly with live Azure Resource Manager and Microsoft Graph state. It supports DNS record sets, security groups, app registrations, role definitions, role assignments, and resource groups. DNS values are compared without ordering sensitivity, resource-group tags use merge semantics, and the live provider remains the source of truth — there is no state backend, initialization step, or local state file.

```text
attune validate              # validate specs offline
attune plan                  # read live state and show changes
attune apply                 # create, update, and permitted prune operations
```

See the [synthetic attune example](../../examples/attune/) for a complete set of six specs. Validation is offline; planning and applying require authentication and deliberate replacement and review of every documented placeholder.

Live commands require an authenticated Azure CLI session from `az login`. Configuration is the store's `attune.yaml` entry; precedence is flag, environment (`ARM_SUBSCRIPTION_ID`, `ARM_RESOURCE_GROUP`), configuration file, then built-in default. DNS pruning defaults to enabled. Identity, role, and resource-group pruning default to disabled and must be enabled explicitly with their corresponding flags or configuration. Role assignments accept `group` and its case-insensitive `securityGroup` alias, `servicePrincipal`, or `user` as `principalType`. A literal directory object ID may omit `principalType`; a named principal must provide it, and `attune validate` enforces this rule offline.

`validate` parses and checks specifications without credentials or provider access. `plan` authenticates and reads live state but does not mutate it. `apply` performs the reviewed create and update actions plus only those deletes enabled by the applicable prune policy. Every live run prints non-secret provider grounding so the operator can confirm its target. Requests are restricted to an ARM/Graph origin allowlist, and non-2xx provider responses are redacted wholesale before reaching any diagnostic or error message.

The change block a live run prints is prospective: change lines and the `N change(s) would be made.` trailer render in yellow, under an uncolored `attune plan: provider=azure` header, for both `plan` and `apply`. `apply` then prints a green confirmation block headed `attune apply: provider=azure` — one green past-tense line (`created`/`updated`/`deleted`) per change, printed as that change lands, so an interrupted run shows exactly what was applied — followed by a green `N change(s) made.` trailer on full success. Color is suppressed automatically for non-TTY output, `NO_COLOR`, and `TERM=dumb`.

DNS zone creation is part of the plan: a spec referencing a zone that does not exist plans as `+ create dnsZone|<zone>` ahead of that zone's record changes, and `apply` creates it through the same reviewed change loop. Existing zones are never rewritten.

An optional `content_version` string in `attune.yaml` declares the content version of the stored specs; when set, attune appends ` content=<value>` to the validate result and to the live grounding line. The convention decouples spec content from repository tags: bump `content_version` when azure-affecting spec content changes, and leave it (and any git tag) alone for unrelated repository changes — attune never reads git.

Normal plans print resource keys and concise summaries, but omit DNS values, tag values, memberships, owners, role actions, credentials, and provider response bodies. `-d`/`--diagnostic` adds non-secret account and target grounding. `-V`/`--verbose` opts in to field-level detail: each planned update gains indented `field: old -> new` lines (added/removed entries for set-valued fields) showing exactly which values drive it — including live tenant values the default output deliberately omits. Resource-group locations are compared by normalized region name, so `East US` in a spec and ARM's `eastus` are the same region, not drift. Apart from its store, the remembered store path, `render` output, and the private temporary file `edit` opens, attune writes no local state, cache, telemetry, copied specs, or diagnostic artifacts; serviced-repository data is sent only to the configured Azure provider endpoints during an operator-requested live command.

## Usage

```text
attune v1.4.0
Reconcile Azure state from YAML specs kept in an encrypted store.

Overview
  The specs live in one sealed store file that attune manages itself: init
  creates or unlocks it, add captures spec files, edit changes a stored spec
  in your editor, and render writes a browsable copy. validate checks the
  stored specs offline, plan reads live Azure state and lists the changes,
  and apply makes them. The key lives in the login keychain, with a
  passphrase-wrapped copy in the store for other Macs.

Usage
  attune (c|validate) [flags]         check the stored specs offline
  attune (p|plan) [flags]             read live state and show the changes
  attune (a|apply) [flags]            create, update, and permitted prune operations
  attune init [-N] [-t PATH]          unlock an existing store, or create one with -N
  attune st                           status of the store, key, and entries
  attune add DIR | NAME [FILE]        import a directory of specs, or store one from a file or stdin
  attune edit NAME                    change a stored spec in $EDITOR and save a validated version
  attune rename OLD NEW               change an entry's name, keeping its versions
  attune rm NAME                      forget an entry and its stored versions
  attune ls [-b FIELD]                list entries by name, or by captured
  attune cat NAME                     print one stored spec
  attune render [-o DIR] [-a] [-f]    write the stored specs into a browsable directory
  attune key show                     store path, key id, keychain and store state
  attune key restore                  put the key back in the keychain with the passphrase
  attune key rm [-f]                  delete the keychain item after a prompt
  attune key passphrase               change the recovery passphrase
  attune (v|version)                  print attune v1.4.0
  attune (h|help)                     show this help

Options
  -t, --store PATH                    Store file for this command; init -t also remembers it
  -P, --provider NAME                 Provider name; azure is the only one
  -S, --subscription ID               Azure subscription for live commands
  -g, --resource-group NAME           Azure resource group for live commands
  -k, --kind KIND                     Limit plan and apply to one spec kind
  -r, --prune[=BOOL]                  Delete unmanaged DNS records; on by default
  -I, --prune-identities[=BOOL]       Delete unmanaged groups and app registrations; off by default
  -R, --prune-roles[=BOOL]            Delete unmanaged role definitions and assignments; off by default
  -G, --prune-resource-groups[=BOOL]  Delete unmanaged resource groups; off by default
  -d, --diagnostic                    Print non-secret account and target grounding on live commands
  -V, --verbose                       Field-level detail on plan and apply
  -N, --new                           Create a new store (init)
  -f, --force                         Render into a non-empty directory; skip the key rm prompt
  -o, --output DIR                    Render into DIR instead of a fresh private temp directory (render)
  -a, --all                           Render every stored version too, under versions/ (render)
  -b, --by FIELD                      Sort ls by name (the default) or captured, newest first
  -v, --version                       Print attune v1.4.0 and exit
  -h, --help                          Show this help message and exit

Notes
  Store path order: -t, then ATTUNE_STORE, then the path init -t remembered in
  $XDG_CONFIG_HOME/attune/store, then $XDG_DATA_HOME/attune/attune.store.
  A NAME is a relative path ending in .yaml or .yml (res/dns/zone.yaml), or
  attune.yaml for the configuration. add and edit validate before saving.
  Drift against Azure is what plan reports; st never contacts Azure.
  Live commands need an authenticated Azure CLI (az login). init and edit
  need a terminal. The store is macOS only.
  render writes plaintext copies of the store; delete the directory when done.
```

## Store

The specs live in one encrypted store that attune manages itself, so they and the secrets inside them stay sealed in a synced folder and no repository needs to hold them. The store is the same kind of file macfit uses, a SQLite database sealed with XChaCha20-Poly1305, but attune keeps its own: its own file, its own key under the `attune` keychain service, its own remembered path, and its own commands. macfit never sees it.

### What it holds

Every entry is one spec named by a relative path ending in `.yaml` or `.yml`, `res/dns/zone.yaml` for example, plus the configuration named `attune.yaml`. The store keeps the YAML text as written, comments included, and every version ever captured. Parsing happens when `validate`, `plan`, or `apply` run, and once more inside `add` and `edit`, which refuse to save anything that would not validate together with the rest of the store.

### Where it is

Each command finds the store in this order: `-t PATH` on the command line, then `ATTUNE_STORE`, then the path `init -t` remembered in `$XDG_CONFIG_HOME/attune/store`, then `$XDG_DATA_HOME/attune/attune.store`. `init -N -t PATH` creates a store at PATH and remembers it; the folder must already exist, so a missing synced folder is never faked. On another Mac, `init -t PATH` asks for the recovery passphrase once and saves the key to that Mac's login keychain. Every command but `help` and `version` needs macOS, because the key lives in the login keychain.

### Working with it

```text
attune init -N -t ~/data/etc/attune-azure.store   # first Mac: create the store and its key, remember the path
attune add attune.yaml ./attune.yaml              # store the configuration under its name
attune add ./specs                                # import every spec under a directory, named relative to it
attune add res/dns/new.yaml < new.yaml            # store one new spec from standard input
attune edit res/dns/new.yaml                      # change a stored spec in $EDITOR; saved only if it validates
attune ls                                         # entries by name; -b captured for newest first
attune cat attune.yaml                            # one stored file on stdout
attune rename dns/old.yaml dns/new.yaml           # change a name, keeping the versions
attune render                                     # browse a plaintext copy in a private temp directory
attune st                                         # store, key, entry count; drift is plan's job
attune init -t ~/data/etc/attune-azure.store      # another Mac: unlock with the passphrase, remember the path
```

`edit` reopens the editor when the result would not validate; quit without changing anything to abort. Drift against Azure is what `plan` reports, so `st` never contacts Azure. `key show`, `key restore`, `key rm`, and `key passphrase` maintain the keychain item exactly as macfit's do; see macfit's README for the key and passphrase model.

### Adding and removing a record

A record's whole life, using a throwaway CNAME. Store it straight from standard input; attune validates it together with every other stored spec before saving, so a typo is refused here rather than at `plan` time:

```text
attune add res/dns/example.com/cname_scratch.yaml <<'EOF'
kind: dnsRecordSet
zone: example.com
type: CNAME
name: scratch
ttl: 300
values:
  - www.example.com
EOF
```

`attune plan` now lists it as the only change, `+ create dnsRecordSet example.com|CNAME|scratch missing`, and `attune apply` creates it. To see it land, ask one of the zone's own name servers rather than a resolver; `dig NS example.com +short` names them:

```text
dig @ns1-01.azure-dns.com scratch.example.com CNAME +short
```

To take it back out, remove the entry and let DNS pruning do the rest: `attune rm res/dns/example.com/cname_scratch.yaml`, after which `attune plan` shows `- delete dnsRecordSet example.com|CNAME|scratch absent from specs` and `attune apply` deletes it. `rm` also drops the entry's stored versions; use `edit` when the record should stay and only its content changes.

### Moving specs from a repository into the store

Done once, from the root of the repository that holds them:

1. `attune init -N -t ~/data/etc/attune-azure.store` and choose a recovery passphrase.
2. `attune add attune.yaml ./attune.yaml`, then `attune add ./specs`, naming whatever directory held the specs.
3. `attune validate` prints the same count and `content=` line the repository copy gave, and `attune plan` lists the same changes.
4. On another Mac, `attune init -t ~/data/etc/attune-azure.store` with the passphrase, then `attune validate`.

Removing the specs from the repository afterwards is a separate step; attune never deletes them.
