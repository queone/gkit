## pman
Call Microsoft Graph and Azure Resource Manager REST APIs with an azm token.

### Why
Calls Microsoft Graph and Azure Resource Manager APIs with a ready token, without building auth headers for `curl` by hand.

### Usage

```text
pman v2.2.0
Call Microsoft Graph and Azure Resource Manager REST APIs with an azm token
github.com/queone/gkit/tree/main/cmd/pman

Usage
  pman METHOD URL [-d DATA]  Send the request with a bearer token and print the response body

  pman needs the azm utility installed and logged in. It asks azm for a
  Microsoft Graph token for graph.microsoft.com URLs and an Azure Resource
  Manager token for management.azure.com URLs.

Options
  -d, --data DATA  Send DATA as the JSON request body
  -v, --version    Print pman v2.2.0 and exit
  -h, -?, --help   Show this help and exit

Examples
  pman GET "https://graph.microsoft.com/v1.0/me"
  pman GET "https://management.azure.com/subscriptions?api-version=2022-04-01"
  pman GET "https://graph.microsoft.com/v1.0/applications/<id>"
```
