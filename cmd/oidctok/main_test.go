package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/queone/gkit/internal/color"
)

const (
	reqToken   = "reqtoken-SECRET-0123456789abcdef"
	armToken   = "armtoken-SECRET-eyJhbGciOiJSUzI1NiJ9.arm"
	graphToken = "graphtoken-SECRET-eyJhbGciOiJSUzI1NiJ9.graph"
)

// fakeJWT builds an unsigned JWT-shaped token carrying claims.
func fakeJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"RS256","typ":"JWT"}`)) + "." + enc(payload) + ".sig"
}

// fakeIssuer stands in for the GitHub OIDC issuer and the Microsoft token endpoint.
type fakeIssuer struct {
	srv  *httptest.Server
	oidc string

	mu           sync.Mutex
	issuerAuth   []string     // Authorization headers seen by the issuer
	issuerAud    []string     // audience query values seen by the issuer
	tokenForms   []url.Values // forms posted to the token endpoint
	issuerStatus int
	failScope    string // scope whose token request fails with 400
	failDesc     string
}

// newFakeIssuer starts a TLS server serving both endpoints and returns it.
func newFakeIssuer(t *testing.T) *fakeIssuer {
	t.Helper()
	f := &fakeIssuer{issuerStatus: http.StatusOK}
	f.oidc = fakeJWT(t, map[string]any{
		"aud": "api://AzureADTokenExchange",
		"iss": "https://token.actions.githubusercontent.com",
		"sub": "repo:org/repo:ref:refs/heads/main",
		"exp": 1757400000, "iat": 1757396400, "nbf": 1757396400,
		"repository": "org/repo",
	})
	mux := http.NewServeMux()
	mux.HandleFunc("/issuer", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.issuerAuth = append(f.issuerAuth, r.Header.Get("Authorization"))
		f.issuerAud = append(f.issuerAud, r.URL.Query().Get("audience"))
		status := f.issuerStatus
		f.mu.Unlock()
		if status != http.StatusOK {
			w.WriteHeader(status)
			w.Write([]byte(`{"message":"Bad credentials"}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"value": f.oidc})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/oauth2/v2.0/token") || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		f.mu.Lock()
		f.tokenForms = append(f.tokenForms, r.PostForm)
		failScope, failDesc := f.failScope, f.failDesc
		f.mu.Unlock()
		scope := r.PostForm.Get("scope")
		if failScope != "" && scope == failScope {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid_client", "error_description": failDesc})
			return
		}
		token := armToken
		if strings.Contains(scope, "graph.microsoft.com") {
			token = graphToken
		}
		json.NewEncoder(w).Encode(map[string]any{"access_token": token, "token_type": "Bearer", "expires_in": 3599})
	})
	f.srv = httptest.NewTLSServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// requests returns how many requests of either kind the server saw.
func (f *fakeIssuer) requests() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.issuerAuth) + len(f.tokenForms)
}

type result struct {
	code   int
	stdout string
	stderr string
}

// runWith runs the utility against the fake issuer with the given variables,
// returning the captured output. vars may override or blank any variable.
func runWith(t *testing.T, f *fakeIssuer, vars map[string]string, args ...string) result {
	t.Helper()
	all := map[string]string{
		"CLIENT_ID":                      "client-1234",
		"TENANT_ID":                      "tenant-5678",
		"ACTIONS_ID_TOKEN_REQUEST_TOKEN": reqToken,
		"ACTIONS_ID_TOKEN_REQUEST_URL":   f.srv.URL + "/issuer",
		"GITHUB_ENV":                     filepath.Join(t.TempDir(), "github_env"),
	}
	maps.Copy(all, vars)
	var out, errBuf bytes.Buffer
	code := run(args, env{
		stdout:        &out,
		stderr:        &errBuf,
		getenv:        func(k string) string { return all[k] },
		client:        f.srv.Client(),
		tokenEndpoint: func(tenant string) string { return f.srv.URL + "/" + tenant + "/oauth2/v2.0/token" },
	})
	return result{code: code, stdout: color.ClearCode(out.String()), stderr: color.ClearCode(errBuf.String())}
}

// githubEnv reads the GITHUB_ENV file for the run, or "" when it was never created.
func githubEnv(t *testing.T, vars map[string]string) string {
	t.Helper()
	b, err := os.ReadFile(vars["GITHUB_ENV"])
	if err != nil {
		return ""
	}
	return string(b)
}

// assertNoSecrets fails when any full secret appears in either output stream.
func assertNoSecrets(t *testing.T, r result, f *fakeIssuer) {
	t.Helper()
	for _, s := range []string{reqToken, armToken, graphToken, f.oidc} {
		if strings.Contains(r.stdout, s) || strings.Contains(r.stderr, s) {
			t.Errorf("output leaks the secret starting %q", s[:8])
		}
	}
}

func TestExchangeWritesBothTokensToGithubEnv(t *testing.T) {
	f := newFakeIssuer(t)
	vars := map[string]string{"GITHUB_ENV": filepath.Join(t.TempDir(), "github_env")}
	r := runWith(t, f, vars)
	if r.code != 0 {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	assertNoSecrets(t, r, f)

	if got := f.issuerAuth; len(got) != 1 || got[0] != "Bearer "+reqToken {
		t.Errorf("issuer Authorization = %v, want one bearer request", got)
	}
	if got := f.issuerAud; len(got) != 1 || got[0] != defaultAudience {
		t.Errorf("issuer audience = %v, want %q", got, defaultAudience)
	}
	if len(f.tokenForms) != 2 {
		t.Fatalf("token requests = %d, want 2", len(f.tokenForms))
	}
	for i, want := range []string{"https://management.azure.com/.default", "https://graph.microsoft.com/.default"} {
		form := f.tokenForms[i]
		checks := map[string]string{
			"grant_type":            "client_credentials",
			"client_id":             "client-1234",
			"scope":                 want,
			"client_assertion_type": assertionType,
			"client_assertion":      f.oidc,
		}
		for k, v := range checks {
			if form.Get(k) != v {
				t.Errorf("token request %d %s = %q, want %q", i, k, form.Get(k), v)
			}
		}
	}
	if got, want := githubEnv(t, vars), "AZ_TOKEN="+armToken+"\nMG_TOKEN="+graphToken+"\n"; got != want {
		t.Errorf("GITHUB_ENV = %q, want %q", got, want)
	}
	for _, s := range []string{"ACTIONS_ID_TOKEN_REQUEST_TOKEN hint = reqt******", "OIDC token hint = eyJh******", "AZ_TOKEN hint = armt******", "MG_TOKEN hint = grap******", "sub: repo:org/repo:ref:refs/heads/main", "exp: 2025-Sep-09 06:40 UTC (1757400000)", "repository: org/repo"} {
		if !strings.Contains(r.stdout, s) {
			t.Errorf("stdout lacks %q:\n%s", s, r.stdout)
		}
	}
}

func TestMissingVariableFailsBeforeAnyRequest(t *testing.T) {
	for _, name := range requiredVars {
		f := newFakeIssuer(t)
		vars := map[string]string{name: "", "GITHUB_ENV": filepath.Join(t.TempDir(), "github_env")}
		if name == "GITHUB_ENV" {
			vars["GITHUB_ENV"] = ""
		}
		r := runWith(t, f, vars)
		if r.code != 1 || !strings.Contains(r.stderr, "variable "+name+" is empty or unset") {
			t.Errorf("%s unset: exit %d, stderr %q", name, r.code, r.stderr)
		}
		if strings.HasPrefix(name, "ACTIONS_") && !strings.Contains(r.stderr, "id-token: write") {
			t.Errorf("%s unset: stderr %q lacks the permissions hint", name, r.stderr)
		}
		if f.requests() != 0 {
			t.Errorf("%s unset: %d requests sent, want 0", name, f.requests())
		}
		assertNoSecrets(t, r, f)
	}
}

func TestTokenEndpointErrorIsReportedTruncated(t *testing.T) {
	f := newFakeIssuer(t)
	f.failScope = "https://management.azure.com/.default"
	f.failDesc = strings.Repeat("d", 2000)
	vars := map[string]string{"GITHUB_ENV": filepath.Join(t.TempDir(), "github_env")}
	r := runWith(t, f, vars)
	if r.code != 1 {
		t.Fatalf("exit %d, want 1; stderr %q", r.code, r.stderr)
	}
	assertNoSecrets(t, r, f)
	if !strings.Contains(r.stderr, "setting AZ_TOKEN") || !strings.Contains(r.stderr, "HTTP 400") {
		t.Errorf("stderr %q lacks the AZ_TOKEN failure", r.stderr)
	}
	if !strings.Contains(r.stderr, strings.Repeat("d", 512)) || strings.Contains(r.stderr, strings.Repeat("d", 513)) {
		t.Errorf("stderr should carry exactly 512 bytes of the description: %q", r.stderr)
	}
	if !strings.Contains(r.stderr, "There were errors") {
		t.Errorf("stderr %q lacks the closing error line", r.stderr)
	}
	if got, want := githubEnv(t, vars), "MG_TOKEN="+graphToken+"\n"; got != want {
		t.Errorf("GITHUB_ENV = %q, want only the Graph line %q", got, want)
	}
}

func TestIssuerFailureIsReported(t *testing.T) {
	f := newFakeIssuer(t)
	f.issuerStatus = http.StatusUnauthorized
	r := runWith(t, f, nil)
	if r.code != 1 || !strings.Contains(r.stderr, "fetching the OIDC token: HTTP 401") || !strings.Contains(r.stderr, "Bad credentials") {
		t.Errorf("exit %d, stderr %q; want exit 1 with the issuer status", r.code, r.stderr)
	}
	if len(f.tokenForms) != 0 {
		t.Errorf("token requests = %d, want 0 after an issuer failure", len(f.tokenForms))
	}
	assertNoSecrets(t, r, f)
}

func TestRejectsPlainHTTPEndpoints(t *testing.T) {
	f := newFakeIssuer(t)
	r := runWith(t, f, map[string]string{"ACTIONS_ID_TOKEN_REQUEST_URL": "http://issuer.example/token"})
	if r.code != 1 || !strings.Contains(r.stderr, "must be an https URL") || f.requests() != 0 {
		t.Errorf("exit %d, stderr %q, requests %d; want exit 1 and no request", r.code, r.stderr, f.requests())
	}
	if _, err := exchange(env{client: f.srv.Client()}, "http://login.example/token", "c", "o", "s"); err == nil || !strings.Contains(err.Error(), "must be an https URL") {
		t.Errorf("exchange over http: err = %v, want an https refusal", err)
	}
}

func TestAudienceFlagOverridesDefault(t *testing.T) {
	for _, args := range [][]string{{"-a", "api://custom"}, {"--audience", "api://custom"}, {"--audience=api://custom"}} {
		f := newFakeIssuer(t)
		if r := runWith(t, f, nil, args...); r.code != 0 {
			t.Fatalf("%v: exit %d, stderr %q", args, r.code, r.stderr)
		}
		if got := f.issuerAud; len(got) != 1 || got[0] != "api://custom" {
			t.Errorf("%v: audience = %v, want api://custom", args, got)
		}
	}
	f := newFakeIssuer(t)
	if r := runWith(t, f, nil, "-a"); r.code != 1 || !strings.Contains(r.stderr, "needs a value") {
		t.Errorf("-a without value: exit %d, stderr %q", r.code, r.stderr)
	}
}

func TestDecodeClaimsParsesPayloadAndRejectsMalformed(t *testing.T) {
	claims, err := decodeClaims(fakeJWT(t, map[string]any{"sub": "x", "exp": 5}))
	if err != nil || claims["sub"] != "x" || claims["exp"] != float64(5) {
		t.Errorf("decodeClaims = %v, %v", claims, err)
	}
	for _, bad := range []string{"", "a.b", "a.b.c.d", "a.!!!.c", "a." + base64.RawURLEncoding.EncodeToString([]byte("not json")) + ".c"} {
		if _, err := decodeClaims(bad); err == nil {
			t.Errorf("decodeClaims(%q) expected an error", bad)
		}
	}
}

func TestPrintClaimsFormatsTimestampsAndSortsKeys(t *testing.T) {
	var b bytes.Buffer
	printClaims(&b, map[string]any{"sub": "s", "aud": "a", "iat": float64(1757396400), "zzz": true, "exp": "not-a-number"})
	got := color.ClearCode(b.String())
	want := "    aud: a\n    exp: not-a-number\n    iat: 2025-Sep-09 05:40 UTC (1757396400)\n    sub: s\n    zzz: true\n"
	if got != want {
		t.Errorf("printClaims =\n%q\nwant\n%q", got, want)
	}
}

func TestHintMasksSecrets(t *testing.T) {
	cases := map[string]string{"": "******", "abc": "******", "abcd": "******", "abcde": "abcd******", reqToken: "reqt******"}
	for in, want := range cases {
		if got := hint(in); got != want {
			t.Errorf("hint(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVersionHelpAndBadArgs(t *testing.T) {
	f := newFakeIssuer(t)
	if r := runWith(t, f, nil, "-v"); r.code != 0 || r.stdout != "oidctok v"+programVersion+"\n" {
		t.Errorf("-v: exit %d stdout %q", r.code, r.stdout)
	}
	for _, flag := range []string{"-h", "--help", "-?"} {
		if r := runWith(t, f, nil, flag); r.code != 0 || !strings.Contains(r.stdout, "\nUsage\n") || !strings.Contains(r.stdout, "-a, --audience") {
			t.Errorf("%s: exit %d stdout %q", flag, r.code, r.stdout)
		}
	}
	if r := runWith(t, f, nil, "extra"); r.code != 1 || !strings.Contains(r.stderr, "unexpected argument") {
		t.Errorf("extra arg: exit %d stderr %q", r.code, r.stderr)
	}
	if f.requests() != 0 {
		t.Errorf("%d requests sent by flag handling, want 0", f.requests())
	}
}

// The help screen follows the shared standard: three header lines, then the
// sections the renderer accepts, with the standard rows left to the renderer.
func TestHelpDocFollowsTheStandard(t *testing.T) {
	if err := helpDoc().Check(); err != nil {
		t.Fatal(err)
	}
}
