# attune
A small declarative reconciler: describe provider-neutral intent in YAML and compare it directly with live Azure Resource Manager and Microsoft Graph state. It supports DNS record sets, security groups, app registrations, role definitions, role assignments, and resource groups. DNS values are compared without ordering sensitivity, resource-group tags use merge semantics, and the live provider remains the source of truth — there is no state backend, initialization step, or local state file.

```text
attune validate              # validate specs offline
attune plan                  # read live state and show changes
attune apply                 # create, update, and permitted prune operations
```

See the [synthetic attune example](../../examples/attune/) for a complete six-resource bundle. Validation is offline; planning and applying require authentication and deliberate replacement and review of every documented placeholder.

Live commands require an authenticated Azure CLI session from `az login`. Configuration is read from the nearest `attune.yaml`; precedence is flag, environment (`ARM_SUBSCRIPTION_ID`, `ARM_RESOURCE_GROUP`), configuration file, then built-in default. DNS pruning defaults to enabled. Identity, role, and resource-group pruning default to disabled and must be enabled explicitly with their corresponding flags or configuration. Role assignments accept `group` and its case-insensitive `securityGroup` alias, `servicePrincipal`, or `user` as `principalType`. A literal directory object ID may omit `principalType`; a named principal must provide it, and `attune validate` enforces this rule offline.

`validate` parses and checks specifications without credentials or provider access. `plan` authenticates and reads live state but does not mutate it. `apply` performs the reviewed create and update actions plus only those deletes enabled by the applicable prune policy. Every live run prints non-secret provider grounding so the operator can confirm its target. Requests are restricted to an ARM/Graph origin allowlist, and non-2xx provider responses are redacted wholesale before reaching any diagnostic or error message.

Normal plans print resource keys and concise summaries, but omit DNS values, tag values, memberships, owners, role actions, credentials, and provider response bodies. `-d`/`--diagnostic` adds non-secret account and target grounding. attune writes no local state, cache, telemetry, copied specs, or diagnostic artifacts; serviced-repository data is sent only to the configured Azure provider endpoints during an operator-requested live command.

## Usage

```text
attune — reconcile provider state with neutral YAML specs.

Usage:
  attune (p|plan) [flags]
  attune (a|apply) [flags]
  attune (c|validate) [flags]
  attune (v|version)
  attune (h|help)

Flags:
  -s, --specs PATH
  -P, --provider NAME
  -g, --resource-group NAME
  -S, --subscription ID
  -k, --kind KIND
  -r, --prune[=BOOL]
  -I, --prune-identities[=BOOL]
  -R, --prune-roles[=BOOL]
  -G, --prune-resource-groups[=BOOL]
  -d, --diagnostic

Live commands require an authenticated Azure CLI (`az login`).
```
