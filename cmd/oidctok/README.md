## oidctok
Exchange a GitHub Actions OIDC token for Azure Resource Manager and Microsoft Graph tokens and hand them to later job steps.

### Why?
A workflow that manages Azure should not carry a client secret. With a federated credential on the app registration, GitHub's OIDC token proves which repository and branch is running, and the Microsoft identity platform trades it for real access tokens. `oidctok` does that exchange in one static binary: no Python, no `pip install`, no Azure CLI login.

```
==> ACTIONS_ID_TOKEN_REQUEST_TOKEN hint = eyJ0******
==> ACTIONS_ID_TOKEN_REQUEST_URL = https://pipelinesghubeus2.actions.githubusercontent.com/...
==> OIDC token hint = eyJh******
==> Decoded OIDC token claims:
    aud: api://AzureADTokenExchange
    exp: 2026-Sep-09 06:40 UTC (1757400000)
    iss: https://token.actions.githubusercontent.com
    sub: repo:org/repo:ref:refs/heads/main
    ...
==> AZ_TOKEN hint = eyJ0******
==> MG_TOKEN hint = eyJ0******
```

### Usage

```text
oidctok v1.1.0
Exchange a GitHub Actions OIDC token for Azure tokens
github.com/queone/gkit/tree/main/cmd/oidctok

Usage
  oidctok [-a AUD]  Fetch the job's OIDC token, print its claims, and exchange it

  The job must define CLIENT_ID and TENANT_ID for the app registration whose
  federated credential trusts this workflow, and grant permissions: id-token: write
  so GitHub sets ACTIONS_ID_TOKEN_REQUEST_TOKEN and ACTIONS_ID_TOKEN_REQUEST_URL.
  oidctok appends AZ_TOKEN (Azure Resource Manager) and MG_TOKEN (Microsoft Graph)
  to the file named by GITHUB_ENV, so later steps see them as variables. Only the
  first four characters of any token are ever printed.

Options
  -a, --audience AUD  Audience for the OIDC token (default api://AzureADTokenExchange)
  -v, --version       Print oidctok v1.1.0 and exit
  -h, -?, --help      Show this help and exit

Examples
  - run: oidctok
    env:
      CLIENT_ID: ${{ vars.CLIENT_ID }}
      TENANT_ID: ${{ vars.TENANT_ID }}
```

Flags: `-a, --audience AUD` overrides the audience requested from GitHub (default `api://AzureADTokenExchange`, which is what an Azure federated credential expects). `-v, --version` and `-h, --help` behave as usual.

### In a workflow
The job needs `permissions: id-token: write`, plus `CLIENT_ID` and `TENANT_ID` for the app registration:

```yaml
permissions:
  id-token: write
  contents: read
steps:
  - run: go install github.com/queone/gkit/cmd/oidctok@latest
  - run: oidctok
    env:
      CLIENT_ID: ${{ vars.CLIENT_ID }}
      TENANT_ID: ${{ vars.TENANT_ID }}
  - run: curl -sS -H "Authorization: Bearer $AZ_TOKEN" https://management.azure.com/subscriptions?api-version=2022-12-01
```

`oidctok` appends `AZ_TOKEN=...` and `MG_TOKEN=...` to the file named by `GITHUB_ENV`, so every later step sees them as environment variables.

### What it prints and what it never prints
Only the first four characters of any token appear in the log, followed by asterisks. The decoded claims of the OIDC token are printed in full because they carry no secret; `aud`, `iss`, and `sub` are the three that must match the federated credential. Every endpoint must be HTTPS, and a failed token response is reported with its error description cut at 512 bytes.

### Python option
`scripts/get_oidc_tokens.py` does the same exchange with the `requests` and `msal` packages, for a job whose runtime is already Python. The Go binary is recommended for simplicity and speed.

### Getting Started
This utility is part of a collection of Go utilities. To compile and install follow the **Getting Started** instructions at the [gkit repo](https://github.com/queone/gkit).
