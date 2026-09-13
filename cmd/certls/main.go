package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/queone/gkit/internal/help"
)

const (
	programName    = "certls"
	programVersion = "2.1.0"
)

func helpText() string {
	return help.Doc{
		Name:        programName,
		Version:     programVersion,
		Description: "Print the TLS certificate details of a host and port",
		URL:         help.URL(programName),
		Sections: []help.Section{
			{Title: "Usage", Rows: []help.Row{{Form: programName + " FQDN[:PORT]", Meaning: "Connect and print the certificate; PORT defaults to 443"}}},
			{Title: "Examples", Rows: []help.Row{
				{Form: programName + " microsoft.com", Meaning: "Uses port 443"},
				{Form: programName + " mysite.com:1473", Meaning: "Uses port 1473"},
			}},
		},
	}.String()
}

// usage prints the help to stderr and exits 1 for a bad command line.
func usage() {
	fmt.Fprint(os.Stderr, helpText())
	os.Exit(1)
}

func parseTarget(arg string) (string, string) {
	if strings.Count(arg, ":") > 1 {
		usage()
	}

	host, port, err := net.SplitHostPort(arg)
	if err == nil {
		return host, port
	}

	// No port specified
	if strings.Contains(arg, ":") {
		usage()
	}
	return arg, "443"
}

func main() {
	if len(os.Args) == 2 {
		switch os.Args[1] {
		case "-v", "--version":
			fmt.Printf("%s v%s\n", programName, programVersion)
			return
		case "-h", "-?", "--help":
			fmt.Print(helpText())
			return
		}
	}

	if len(os.Args) != 2 {
		usage()
	}

	host, port := parseTarget(os.Args[1])
	if host == "" {
		usage()
	}

	address := net.JoinHostPort(host, port)

	conn, err := tls.Dial("tcp", address, &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: false,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "TLS connection failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		fmt.Fprintln(os.Stderr, "No certificates presented by server")
		os.Exit(1)
	}

	cert := state.PeerCertificates[0]

	fmt.Printf("==> Go TLS version %s\n", runtimeVersion())
	fmt.Printf("==> FQDN:Port %s:%s\n", host, port)
	fmt.Printf("==> EXPIRY: NotBefore=%s NotAfter=%s\n",
		cert.NotBefore.Format(time.RFC3339),
		cert.NotAfter.Format(time.RFC3339),
	)

	fmt.Println("==> LIST")
	for _, dns := range cert.DNSNames {
		fmt.Println(dns)
	}
}

func runtimeVersion() string {
	if v := os.Getenv("GOVERSION"); v != "" {
		return v
	}
	return strings.TrimPrefix(runtime.Version(), "go")
}
