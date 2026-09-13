package main

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/queone/gkit/internal/help"
)

const (
	programName    = "certgen"
	programVersion = "2.1.0"
)

func helpText() string {
	return help.Doc{
		Name:        programName,
		Version:     programVersion,
		Description: "Generate a self-signed TLS certificate, private key, and CSR for a common name",
		URL:         help.URL(programName),
		Sections: []help.Section{
			{Title: "Usage", Rows: []help.Row{{Form: programName + " COMMON_NAME", Meaning: "Write COMMON_NAME.key, .csr, and .crt here after a confirmation prompt"}}},
			{Title: "Examples", Rows: []help.Row{{Form: programName + " www.example.com", Meaning: ""}}},
		},
	}.String()
}

// usage prints the help to stderr and exits 1 for a bad command line.
func usage() {
	fmt.Fprint(os.Stderr, helpText())
	os.Exit(1)
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

	fqdn := strings.TrimSpace(os.Args[1])
	if fqdn == "" {
		usage()
	}

	// Static org info (matches Bash script)
	subject := pkix.Name{
		Country:            []string{"US"},
		Province:           []string{"NY"},
		Locality:           []string{"New York"},
		Organization:       []string{"My Org"},
		OrganizationalUnit: []string{"My Unit"},
		CommonName:         fqdn,
	}

	fmt.Printf("\nCOUNTRY=%s   STATE=%s   LOC=%s   ORG=%s   UNIT=%s   DOMAIN=%s\n\n",
		subject.Country[0],
		subject.Province[0],
		subject.Locality[0],
		subject.Organization[0],
		subject.OrganizationalUnit[0],
		fqdn,
	)

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Proceed to create a self-signed cert + key, with a CSR for domain '%s'? Y/N ", fqdn)
	resp, _ := reader.ReadString('\n')
	resp = strings.TrimSpace(resp)
	if resp != "Y" && resp != "y" {
		fmt.Println("\nAborted.")
		os.Exit(1)
	}

	fmt.Printf("\nGenerating private key, self-signed cert, and CSR ...\n")

	// Generate RSA key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fail(err)
	}

	// Write private key
	writePEM(fqdn+".key", "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key), 0600)

	// Create CSR
	csrTemplate := x509.CertificateRequest{
		Subject:  subject,
		DNSNames: []string{fqdn},
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &csrTemplate, key)
	if err != nil {
		fail(err)
	}
	writePEM(fqdn+".csr", "CERTIFICATE REQUEST", csrDER, 0644)

	// Self-signed cert (10 years)
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	certTemplate := x509.Certificate{
		SerialNumber: serial,
		Subject:      subject,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),

		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{fqdn},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &certTemplate, &certTemplate, &key.PublicKey, key)
	if err != nil {
		fail(err)
	}
	writePEM(fqdn+".crt", "CERTIFICATE", certDER, 0644)

	fmt.Printf("\n1) You may use the self-signed cert + private key\n")
	fmt.Printf("2) Or submit the CSR to a public CA (Entrust, etc.)\n")

	listFiles(fqdn)
}

func writePEM(path, pemType string, der []byte, mode os.FileMode) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		fail(err)
	}
	defer f.Close()

	if err := pem.Encode(f, &pem.Block{Type: pemType, Bytes: der}); err != nil {
		fail(err)
	}
}

func listFiles(prefix string) {
	entries, _ := os.ReadDir(".")
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix+".") {
			info, _ := e.Info()
			fmt.Printf("%s\t%d bytes\n", e.Name(), info.Size())
		}
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
