## certgen
Generate a self-signed TLS certificate, private key, and CSR for a common name.

### Why
Creates a self-signed certificate, private key, and CSR in one step, without remembering long `openssl` commands.

### Usage

```text
certgen v2.2.0
Generate a self-signed TLS certificate, private key, and CSR for a common name
github.com/queone/gkit/tree/main/cmd/certgen

Usage
  certgen COMMON_NAME  Write COMMON_NAME.key, .csr, and .crt here after a confirmation prompt

Options
  -v, --version   Print certgen v2.2.0 and exit
  -h, -?, --help  Show this help and exit

Examples
  certgen www.example.com
```
