package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/queone/gkit/internal/color"
)

func TestVersionHelpAndBadArguments(t *testing.T) {
	if r := runWith(t, newFake(), nil, "-v"); r.code != 0 || r.stdout != "tfe v"+programVersion+"\n" {
		t.Errorf("-v: exit %d stdout %q", r.code, r.stdout)
	}
	if r := runWith(t, newFake(), nil, "--version"); r.stdout != "tfe v"+programVersion+"\n" {
		t.Errorf("--version: stdout %q", r.stdout)
	}
	if r := runWith(t, newFake(), nil, "version"); r.code != 0 || r.stdout != "tfe v"+programVersion+"\n" {
		t.Errorf("version word: exit %d stdout %q", r.code, r.stdout)
	}
	for _, args := range [][]string{{}, {"-h"}, {"--help"}, {"-?"}, {"help"}} {
		if r := runWith(t, newFake(), nil, args...); r.code != 0 || !strings.Contains(r.stdout, "\nUsage\n") || !strings.Contains(r.stdout, "clone SRC DEST") {
			t.Errorf("%v: exit %d stdout %q", args, r.code, r.stdout)
		}
	}
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"bogus"}, `unknown subcommand "bogus"`},
		{[]string{"orgs", "--bogus"}, `unknown flag "--bogus"`},
		{[]string{"show"}, "show needs 1 argument"},
		{[]string{"clone", "a"}, "clone needs 2 argument"},
		{[]string{"orgs", "a", "b"}, "orgs takes at most one FILTER"},
		{[]string{"ws", "-a"}, "apply to mods only"},
		{[]string{"-j", "orgs"}, "apply to mods only"},
	}
	for _, c := range cases {
		if r := runWith(t, newFake(), nil, c.args...); r.code != 1 || !strings.Contains(r.stderr, c.want) {
			t.Errorf("%v: exit %d stderr %q, want %q", c.args, r.code, r.stderr, c.want)
		}
	}
}

func TestFlagsAcceptShortAndLongForms(t *testing.T) {
	for _, args := range [][]string{{"mods", "-a", "net"}, {"mods", "--all", "net"}, {"mods", "net", "-a"}} {
		if r := runWith(t, newFake(), nil, args...); r.code != 0 || strings.Count(r.stdout, "\n") != 4 {
			t.Errorf("%v: exit %d stdout %q", args, r.code, r.stdout)
		}
	}
	for _, args := range [][]string{{"mods", "-j", "database"}, {"mods", "--json", "database"}} {
		if r := runWith(t, newFake(), nil, args...); r.code != 0 || !strings.HasPrefix(r.stdout, "{") {
			t.Errorf("%v: exit %d stdout %q", args, r.code, r.stdout)
		}
	}
}

func TestRealClientSendsTheTokenToTheConfiguredAddress(t *testing.T) {
	t.Setenv("TFE_TOKEN", "")
	t.Setenv("TFE_ADDRESS", "")
	var auth, path string
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/terraform.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tfe.v2":"/api/v2/"}`))
	})
	mux.HandleFunc("/api/v2/organizations", func(w http.ResponseWriter, r *http.Request) {
		auth, path = r.Header.Get("Authorization"), r.URL.Path
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.Write([]byte(`{"data":[{"id":"acme","type":"organizations","attributes":{"name":"acme"}}],` +
			`"meta":{"pagination":{"current-page":1,"prev-page":null,"next-page":null,"total-pages":1,"total-count":1}}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	vars := map[string]string{"TF_ORG": "acme", "TF_DOMAIN": srv.URL, "TF_TOKEN": "tok-WIRE-SECRET"}
	var out, errBuf bytes.Buffer
	code := run([]string{"orgs"}, env{
		stdout:     &out,
		stderr:     &errBuf,
		getenv:     func(k string) string { return vars[k] },
		configPath: filepath.Join(t.TempDir(), "config.yaml"),
		newClient:  newClient,
	})
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, errBuf.String())
	}
	if auth != "Bearer tok-WIRE-SECRET" || path != "/api/v2/organizations" {
		t.Errorf("request: auth %q path %q", auth, path)
	}
	if got := color.ClearCode(out.String()); got != "acme\n" {
		t.Errorf("stdout %q", got)
	}
}

// The help screen follows the shared standard: three header lines, then the
// sections the renderer accepts, with the standard rows left to the renderer.
func TestHelpDocFollowsTheStandard(t *testing.T) {
	if err := helpDoc().Check(); err != nil {
		t.Fatal(err)
	}
}
