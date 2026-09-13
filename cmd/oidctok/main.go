// main.go

package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/queone/gkit/internal/color"
	"github.com/queone/gkit/internal/help"
)

const (
	programName    = "oidctok"
	programVersion = "1.1.0"

	defaultAudience = "api://AzureADTokenExchange"
	assertionType   = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"
	requestTimeout  = 30 * time.Second
	bodyLimit       = 512     // bytes of an error body carried into a message
	readLimit       = 1 << 20 // bytes read from any response
)

// scopeTarget pairs an Azure scope with the variable that receives its token.
type scopeTarget struct {
	envVar string
	scope  string
}

// targets lists the tokens written, in order: Azure Resource Manager, then Microsoft Graph.
var targets = []scopeTarget{
	{envVar: "AZ_TOKEN", scope: "https://management.azure.com/.default"},
	{envVar: "MG_TOKEN", scope: "https://graph.microsoft.com/.default"},
}

// requiredVars lists the variables the job must define, in the order they are checked.
var requiredVars = []string{
	"CLIENT_ID",
	"TENANT_ID",
	"ACTIONS_ID_TOKEN_REQUEST_TOKEN",
	"ACTIONS_ID_TOKEN_REQUEST_URL",
	"GITHUB_ENV",
}

// env carries the streams, the environment, the HTTP client, and the token
// endpoint builder so tests can replace every outside contact.
type env struct {
	stdout        io.Writer
	stderr        io.Writer
	getenv        func(string) string
	client        *http.Client
	tokenEndpoint func(tenant string) string
}

// defaultTokenEndpoint returns the Microsoft identity platform v2 token endpoint for tenant.
func defaultTokenEndpoint(tenant string) string {
	return "https://login.microsoftonline.com/" + url.PathEscape(tenant) + "/oauth2/v2.0/token"
}

// usage returns the help screen.
// helpDoc describes the help screen.
func helpDoc() help.Doc {
	return help.Doc{
		Name:        programName,
		Version:     programVersion,
		Description: "Exchange a GitHub Actions OIDC token for Azure tokens",
		URL:         help.URL(programName),
		Sections: []help.Section{
			{Title: "Usage", Rows: []help.Row{{Form: programName + " [-a AUD]", Meaning: "Fetch the job's OIDC token, print its claims, and exchange it"}},
				Lines: []string{
					"The job must define CLIENT_ID and TENANT_ID for the app registration whose",
					"federated credential trusts this workflow, and grant permissions: id-token: write",
					"so GitHub sets ACTIONS_ID_TOKEN_REQUEST_TOKEN and ACTIONS_ID_TOKEN_REQUEST_URL.",
					"oidctok appends AZ_TOKEN (Azure Resource Manager) and MG_TOKEN (Microsoft Graph)",
					"to the file named by GITHUB_ENV, so later steps see them as variables. Only the",
					"first four characters of any token are ever printed.",
				}},
			{Title: "Options", Rows: []help.Row{{Form: "-a, --audience AUD", Meaning: "Audience for the OIDC token (default " + defaultAudience + ")"}}},
			{Title: "Examples", Lines: []string{
				"- run: oidctok",
				"  env:",
				"    CLIENT_ID: ${{ vars.CLIENT_ID }}",
				"    TENANT_ID: ${{ vars.TENANT_ID }}",
			}},
		},
	}
}

// usage returns the help screen.
func usage() string { return helpDoc().String() }

// fail prints err with the program name and returns the failure exit code.
func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "%s: %v\n", programName, err)
	return 1
}

// hint returns the first four characters of a secret followed by asterisks, or
// only asterisks for anything shorter, so logs never carry a usable value.
func hint(s string) string {
	if len(s) <= 4 {
		return "******"
	}
	return s[:4] + "******"
}

// truncate cuts s to at most n bytes.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// requireHTTPS rejects any endpoint that is not an https URL with a host.
func requireHTTPS(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, fmt.Errorf("endpoint %q must be an https URL", raw)
	}
	return u, nil
}

// requireVars reads every variable the job must define and reports the first missing one.
func requireVars(getenv func(string) string) (map[string]string, error) {
	vars := make(map[string]string, len(requiredVars))
	for _, name := range requiredVars {
		v := getenv(name)
		if v == "" {
			extra := ""
			if strings.HasPrefix(name, "ACTIONS_") {
				extra = " (does the workflow grant permissions: id-token: write?)"
			}
			return nil, fmt.Errorf("variable %s is empty or unset%s", name, extra)
		}
		vars[name] = v
	}
	return vars, nil
}

// errorBody reads a failed response and returns its error_description when the
// body is the usual JSON error, otherwise the raw text, cut to bodyLimit bytes.
func errorBody(r io.Reader) string {
	raw, _ := io.ReadAll(io.LimitReader(r, readLimit))
	var body struct {
		ErrorDescription string `json:"error_description"`
	}
	if json.Unmarshal(raw, &body) == nil && body.ErrorDescription != "" {
		return truncate(body.ErrorDescription, bodyLimit)
	}
	return truncate(strings.TrimSpace(string(raw)), bodyLimit)
}

// fetchOIDCToken asks the GitHub Actions issuer for the job's OIDC token for audience.
func fetchOIDCToken(e env, reqURL, reqToken, audience string) (string, error) {
	u, err := requireHTTPS(reqURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("audience", audience)
	u.RawQuery = q.Encode()
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("building the OIDC token request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+reqToken)
	req.Header.Set("Accept", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching the OIDC token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching the OIDC token: HTTP %d: %s", resp.StatusCode, errorBody(resp.Body))
	}
	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, readLimit)).Decode(&body); err != nil {
		return "", fmt.Errorf("fetching the OIDC token: reading the response: %w", err)
	}
	if body.Value == "" {
		return "", errors.New("fetching the OIDC token: the response carried no value")
	}
	return body.Value, nil
}

// decodeClaims parses the payload of a JWT without verifying it; the token is
// displayed here, never trusted.
func decodeClaims(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid OIDC token format: expected three dot-separated parts")
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return nil, fmt.Errorf("decoding the OIDC token payload: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("parsing the OIDC token payload: %w", err)
	}
	return claims, nil
}

// printClaims lists the claims sorted by name: aud, iss, and sub in yellow, the
// three timestamps in green with a readable UTC date, everything else plain.
func printClaims(w io.Writer, claims map[string]any) {
	keys := make([]string, 0, len(claims))
	for k := range claims {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := claims[k]
		switch k {
		case "aud", "iss", "sub":
			fmt.Fprintln(w, color.Yel5(fmt.Sprintf("    %s: %v", k, v)))
		case "exp", "nbf", "iat":
			if f, ok := v.(float64); ok {
				ts := time.Unix(int64(f), 0).UTC().Format("2006-Jan-02 15:04")
				fmt.Fprintln(w, color.Grn5(fmt.Sprintf("    %s: %s UTC (%d)", k, ts, int64(f))))
				continue
			}
			fmt.Fprintf(w, "    %s: %v\n", k, v)
		default:
			fmt.Fprintf(w, "    %s: %v\n", k, v)
		}
	}
}

// exchange trades the OIDC token for an access token covering scope, using the
// token as the client assertion of a client-credentials grant.
func exchange(e env, endpoint, clientID, oidcToken, scope string) (string, error) {
	if _, err := requireHTTPS(endpoint); err != nil {
		return "", err
	}
	form := url.Values{
		"client_id":             {clientID},
		"grant_type":            {"client_credentials"},
		"scope":                 {scope},
		"client_assertion_type": {assertionType},
		"client_assertion":      {oidcToken},
	}
	resp, err := e.client.PostForm(endpoint, form)
	if err != nil {
		return "", fmt.Errorf("requesting a token for %s: %w", scope, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("requesting a token for %s: HTTP %d: %s", scope, resp.StatusCode, errorBody(resp.Body))
	}
	var body struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, readLimit)).Decode(&body); err != nil {
		return "", fmt.Errorf("requesting a token for %s: reading the response: %w", scope, err)
	}
	if body.AccessToken == "" {
		return "", fmt.Errorf("requesting a token for %s: the response carried no access_token", scope)
	}
	if strings.ContainsAny(body.AccessToken, "\r\n") {
		return "", fmt.Errorf("requesting a token for %s: the access_token contains a line break", scope)
	}
	return body.AccessToken, nil
}

// appendEnv appends NAME=value to the GITHUB_ENV file, creating it if needed.
func appendEnv(path, name, value string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("opening the GITHUB_ENV file %q: %w", path, err)
	}
	if _, err := fmt.Fprintf(f, "%s=%s\n", name, value); err != nil {
		f.Close()
		return fmt.Errorf("writing %s to %q: %w", name, path, err)
	}
	return f.Close()
}

// run executes the utility with the given arguments and returns its exit code.
func run(args []string, e env) int {
	audience := defaultAudience
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-v" || a == "--version":
			fmt.Fprintf(e.stdout, "%s v%s\n", programName, programVersion)
			return 0
		case a == "-h" || a == "-?" || a == "--help":
			fmt.Fprint(e.stdout, usage())
			return 0
		case a == "-a" || a == "--audience":
			if i+1 >= len(args) {
				return fail(e.stderr, errors.New("-a/--audience needs a value"))
			}
			i++
			audience = args[i]
		case strings.HasPrefix(a, "--audience="):
			audience = strings.TrimPrefix(a, "--audience=")
		default:
			return fail(e.stderr, fmt.Errorf("unexpected argument %q (see %s --help)", a, programName))
		}
	}
	if audience == "" {
		return fail(e.stderr, errors.New("audience must not be empty"))
	}

	vars, err := requireVars(e.getenv)
	if err != nil {
		return fail(e.stderr, err)
	}
	reqToken := vars["ACTIONS_ID_TOKEN_REQUEST_TOKEN"]
	reqURL := vars["ACTIONS_ID_TOKEN_REQUEST_URL"]
	fmt.Fprintln(e.stdout, color.Yel5("==> ACTIONS_ID_TOKEN_REQUEST_TOKEN hint = "+hint(reqToken)))
	fmt.Fprintln(e.stdout, color.Yel5("==> ACTIONS_ID_TOKEN_REQUEST_URL = "+reqURL))

	oidcToken, err := fetchOIDCToken(e, reqURL, reqToken, audience)
	if err != nil {
		return fail(e.stderr, err)
	}
	fmt.Fprintln(e.stdout, color.Yel5("==> OIDC token hint = "+hint(oidcToken)))
	claims, err := decodeClaims(oidcToken)
	if err != nil {
		return fail(e.stderr, err)
	}
	fmt.Fprintln(e.stdout, color.Yel5("==> Decoded OIDC token claims:"))
	printClaims(e.stdout, claims)

	failed := false
	for _, t := range targets {
		token, err := exchange(e, e.tokenEndpoint(vars["TENANT_ID"]), vars["CLIENT_ID"], oidcToken, t.scope)
		if err != nil {
			fmt.Fprintf(e.stderr, "%s: setting %s: %v\n", programName, t.envVar, err)
			failed = true
			continue
		}
		if err := appendEnv(vars["GITHUB_ENV"], t.envVar, token); err != nil {
			fmt.Fprintf(e.stderr, "%s: setting %s: %v\n", programName, t.envVar, err)
			failed = true
			continue
		}
		fmt.Fprintln(e.stdout, color.Yel5(fmt.Sprintf("==> %s hint = %s", t.envVar, hint(token))))
	}
	if failed {
		fmt.Fprintln(e.stderr, color.Red5("==> There were errors acquiring or setting the tokens."))
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], env{
		stdout:        os.Stdout,
		stderr:        os.Stderr,
		getenv:        os.Getenv,
		client:        &http.Client{Timeout: requestTimeout},
		tokenEndpoint: defaultTokenEndpoint,
	}))
}
