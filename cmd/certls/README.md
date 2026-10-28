## certls
Print the TLS certificate details of a host and port.

### Why
Shows a server's certificate details in readable form, without piping `openssl s_client` into `openssl x509`.

### Usage

```text
certls v2.2.0
Print the TLS certificate details of a host and port
github.com/queone/gkit/tree/main/cmd/certls

Usage
  certls FQDN[:PORT]  Connect and print the certificate; PORT defaults to 443

Options
  -v, --version   Print certls v2.2.0 and exit
  -h, -?, --help  Show this help and exit

Examples
  certls microsoft.com    Uses port 443
  certls mysite.com:1473  Uses port 1473
```
