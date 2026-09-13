package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/queone/gkit/internal/help"
)

const (
	programName    = "pman"
	programVersion = "2.1.0"
)

func usage() string {
	return help.Doc{
		Name:        programName,
		Version:     programVersion,
		Description: "Call Microsoft Graph and Azure Resource Manager REST APIs with an azm token",
		URL:         help.URL(programName),
		Sections: []help.Section{
			{Title: "Usage", Rows: []help.Row{{Form: programName + " METHOD URL [-d DATA]", Meaning: "Send the request with a bearer token and print the response body"}},
				Lines: []string{
					"pman needs the azm utility installed and logged in. It asks azm for a",
					"Microsoft Graph token for graph.microsoft.com URLs and an Azure Resource",
					"Manager token for management.azure.com URLs.",
				}},
			{Title: "Options", Rows: []help.Row{{Form: "-d, --data DATA", Meaning: "Send DATA as the JSON request body"}}},
			{Title: "Examples", Rows: []help.Row{
				{Form: programName + ` GET "https://graph.microsoft.com/v1.0/me"`, Meaning: ""},
				{Form: programName + ` GET "https://management.azure.com/subscriptions?api-version=2022-04-01"`, Meaning: ""},
				{Form: programName + ` GET "https://graph.microsoft.com/v1.0/applications/<id>"`, Meaning: ""},
			}},
		},
	}.String()
}

// printUsage prints the help to stderr and exits 1 for a bad command line.
func printUsage() {
	fmt.Fprint(os.Stderr, usage())
	os.Exit(1)
}

func checkBinary(name string) {
	if _, err := exec.LookPath(name); err != nil {
		fmt.Fprintf(os.Stderr, "Missing '%s' binary!\n", name)
		os.Exit(1)
	}
}

func getToken(url string) string {
	var cmd *exec.Cmd

	switch {
	case strings.Contains(url, "https://graph.microsoft.com"):
		cmd = exec.Command("azm", "-tmg")
	case strings.Contains(url, "https://management.azure.com"):
		cmd = exec.Command("azm", "-taz")
	default:
		printUsage()
	}

	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to obtain token: %v\n", err)
		os.Exit(1)
	}

	token := strings.TrimSpace(string(out))

	if !strings.HasPrefix(token, "eyJ") {
		fmt.Fprintln(os.Stderr) // blank line
		fmt.Fprintln(os.Stderr, "WARNING: Token string is invalid!")
		fmt.Fprintln(os.Stderr) // blank line
	}

	return token
}

func main() {
	if len(os.Args) == 2 {
		switch os.Args[1] {
		case "-v", "--version":
			fmt.Printf("%s v%s\n", programName, programVersion)
			return
		case "-h", "-?", "--help":
			fmt.Print(usage())
			return
		}
	}

	checkBinary("azm")

	if len(os.Args) < 3 {
		printUsage()
	}

	method := strings.ToUpper(os.Args[1])
	url := os.Args[2]
	extraArgs := os.Args[3:]

	token := getToken(url)

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request creation failed: %v\n", err)
		os.Exit(1)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	// Handle optional curl-style flags minimally:
	// Only supports: -d / --data
	for i := 0; i < len(extraArgs); i++ {
		if extraArgs[i] == "-d" || extraArgs[i] == "--data" {
			if i+1 >= len(extraArgs) {
				fmt.Fprintln(os.Stderr, "Missing value for -d/--data")
				os.Exit(1)
			}
			req.Body = io.NopCloser(bytes.NewBufferString(extraArgs[i+1]))
			i++
		}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "HTTP request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Print(string(body))
}
