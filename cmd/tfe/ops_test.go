package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-tfe"
	"github.com/queone/gkit/color"
)

// fakeAPI serves canned pages and records every mutation.
type fakeAPI struct {
	orgPages    [][]*tfe.Organization
	modPages    [][]*tfe.RegistryModule
	versions    map[string]*tfe.RegistryModuleVersion // name@version
	wsPages     [][]*tfe.Workspace
	workspaces  map[string]*tfe.Workspace // by name
	varPages    map[string][][]*tfe.Variable
	pools       map[string]*tfe.AgentPool
	failOp      string // operation name that returns an error
	created     []tfe.WorkspaceCreateOptions
	createdVars map[string][]tfe.VariableCreateOptions
}

var errBoom = errors.New("boom")

// page returns one page of items and the following page number.
func page[T any](pages [][]T, p int) ([]T, int) {
	if p < 1 || p > len(pages) {
		return nil, 0
	}
	if p == len(pages) {
		return pages[p-1], 0
	}
	return pages[p-1], p + 1
}

func (f *fakeAPI) fail(op string) error {
	if f.failOp == op {
		return errBoom
	}
	return nil
}

func (f *fakeAPI) ListOrganizations(_ context.Context, p int) ([]*tfe.Organization, int, error) {
	if err := f.fail("orgs"); err != nil {
		return nil, 0, err
	}
	items, nextPage := page(f.orgPages, p)
	return items, nextPage, nil
}

func (f *fakeAPI) ListModules(_ context.Context, _ string, p int) ([]*tfe.RegistryModule, int, error) {
	if err := f.fail("mods"); err != nil {
		return nil, 0, err
	}
	items, nextPage := page(f.modPages, p)
	return items, nextPage, nil
}

func (f *fakeAPI) ReadModuleVersion(_ context.Context, id tfe.RegistryModuleID, version string) (*tfe.RegistryModuleVersion, error) {
	if v, ok := f.versions[id.Name+"@"+version]; ok {
		return v, nil
	}
	return nil, errBoom
}

func (f *fakeAPI) ListWorkspaces(_ context.Context, _ string, p int) ([]*tfe.Workspace, int, error) {
	if err := f.fail("ws"); err != nil {
		return nil, 0, err
	}
	items, nextPage := page(f.wsPages, p)
	return items, nextPage, nil
}

func (f *fakeAPI) ReadWorkspace(_ context.Context, _, name string) (*tfe.Workspace, error) {
	if err := f.fail("read"); err != nil {
		return nil, err
	}
	if ws, ok := f.workspaces[name]; ok {
		return ws, nil
	}
	return nil, tfe.ErrResourceNotFound
}

func (f *fakeAPI) CreateWorkspace(_ context.Context, _ string, options tfe.WorkspaceCreateOptions) (*tfe.Workspace, error) {
	if err := f.fail("create"); err != nil {
		return nil, err
	}
	f.created = append(f.created, options)
	return &tfe.Workspace{ID: "ws-new", Name: *options.Name}, nil
}

func (f *fakeAPI) ListVariables(_ context.Context, workspaceID string, p int) ([]*tfe.Variable, int, error) {
	if err := f.fail("vars"); err != nil {
		return nil, 0, err
	}
	items, nextPage := page(f.varPages[workspaceID], p)
	return items, nextPage, nil
}

func (f *fakeAPI) CreateVariable(_ context.Context, workspaceID string, options tfe.VariableCreateOptions) (*tfe.Variable, error) {
	if err := f.fail("createvar"); err != nil {
		return nil, err
	}
	if f.createdVars == nil {
		f.createdVars = map[string][]tfe.VariableCreateOptions{}
	}
	f.createdVars[workspaceID] = append(f.createdVars[workspaceID], options)
	return &tfe.Variable{Key: *options.Key}, nil
}

func (f *fakeAPI) ReadAgentPool(_ context.Context, id string) (*tfe.AgentPool, error) {
	if pool, ok := f.pools[id]; ok {
		return pool, nil
	}
	return nil, tfe.ErrResourceNotFound
}

// newFake builds a two-page fixture for every list.
func newFake() *fakeAPI {
	day := time.Date(2026, time.September, 8, 9, 30, 0, 0, time.UTC)
	network := &tfe.RegistryModule{ID: "mod-net", Name: "network", Provider: "azurerm", Namespace: "acme", RegistryName: tfe.PrivateRegistry,
		CreatedAt: "2026-01-02T03:04:05Z", UpdatedAt: "2026-02-03T04:05:06Z",
		VCSRepo:         &tfe.VCSRepo{RepositoryHTTPURL: "https://github.com/acme/terraform-azurerm-network"},
		VersionStatuses: []tfe.RegistryModuleVersionStatuses{{Version: "1.2.0"}, {Version: "1.10.0"}, {Version: "1.9.0"}}}
	netapp := &tfe.RegistryModule{ID: "mod-netapp", Name: "netapp", Provider: "azurerm", Namespace: "hashicorp", RegistryName: tfe.PublicRegistry,
		VersionStatuses: []tfe.RegistryModuleVersionStatuses{{Version: "0.1.0"}}}
	database := &tfe.RegistryModule{ID: "mod-db", Name: "database", Provider: "azurerm", Namespace: "acme", RegistryName: tfe.PrivateRegistry,
		CreatedAt: "2026-03-04T05:06:07Z", UpdatedAt: "2026-04-05T06:07:08Z",
		VersionStatuses: []tfe.RegistryModuleVersionStatuses{{Version: "2.0.0"}}}
	prod := &tfe.Workspace{ID: "ws-1", Name: "prod-network", Description: "Production network", TerraformVersion: "1.9.5", AutoApply: true,
		WorkingDirectory: "", ExecutionMode: "agent", CreatedAt: day, UpdatedAt: day.Add(time.Hour), AgentPool: &tfe.AgentPool{ID: "apool-1"}}
	staging := &tfe.Workspace{ID: "ws-2", Name: "staging-db", ExecutionMode: "remote", CreatedAt: day, UpdatedAt: day}
	dev := &tfe.Workspace{ID: "ws-3", Name: "dev-network", ExecutionMode: "remote", CreatedAt: day, UpdatedAt: day}
	return &fakeAPI{
		orgPages: [][]*tfe.Organization{{{Name: "acme"}, {Name: "Acme-Labs"}}, {{Name: "other"}}},
		modPages: [][]*tfe.RegistryModule{{network, netapp}, {database}},
		versions: map[string]*tfe.RegistryModuleVersion{
			"network@1.2.0":  {ID: "modver-1", Version: "1.2.0", CreatedAt: "2026-01-02T03:04:05Z", UpdatedAt: "2026-01-02T03:04:05Z"},
			"network@1.10.0": {ID: "modver-3", Version: "1.10.0", CreatedAt: "2026-02-03T04:05:06Z", UpdatedAt: "2026-02-03T04:05:06Z"},
			"network@1.9.0":  {ID: "modver-2", Version: "1.9.0", CreatedAt: "2026-01-20T00:00:00Z", UpdatedAt: "2026-01-21T00:00:00Z"},
			"netapp@0.1.0":   {ID: "modver-4", Version: "0.1.0", CreatedAt: "2025-12-01T00:00:00Z", UpdatedAt: "2025-12-01T00:00:00Z"},
			"database@2.0.0": {ID: "modver-5", Version: "2.0.0", CreatedAt: "2026-03-04T05:06:07Z", UpdatedAt: "2026-04-05T06:07:08Z"},
		},
		wsPages:    [][]*tfe.Workspace{{prod, staging}, {dev}},
		workspaces: map[string]*tfe.Workspace{"prod-network": prod, "staging-db": staging, "dev-network": dev},
		varPages: map[string][][]*tfe.Variable{"ws-1": {
			{{Key: "ARM_CLIENT_SECRET", Value: "", Category: tfe.CategoryEnv, Sensitive: true}, {Key: "region", Value: "eastus", Category: tfe.CategoryTerraform}},
			{{Key: "tags", Value: `{team="net"}`, Category: tfe.CategoryTerraform, HCL: true}},
		}},
		pools: map[string]*tfe.AgentPool{"apool-1": {ID: "apool-1", Name: "on-prem-agents"}},
	}
}

type result struct {
	code   int
	stdout string
	stderr string
}

// runWith runs the utility against the fake with environment credentials set,
// clearing the variables go-tfe reads on its own. vars override any variable.
func runWith(t *testing.T, f *fakeAPI, vars map[string]string, args ...string) result {
	t.Helper()
	t.Setenv("TFE_TOKEN", "")
	t.Setenv("TFE_ADDRESS", "")
	all := map[string]string{"TF_ORG": "acme", "TF_DOMAIN": "https://tfe.example.com", "TF_TOKEN": "tok-SECRET-1234567890"}
	maps.Copy(all, vars)
	var out, errBuf bytes.Buffer
	code := run(args, env{
		stdout:     &out,
		stderr:     &errBuf,
		getenv:     func(k string) string { return all[k] },
		configPath: filepath.Join(t.TempDir(), "config.yaml"),
		newClient:  func(string, string) (api, error) { return f, nil },
	})
	return result{code: code, stdout: color.ClearCode(out.String()), stderr: color.ClearCode(errBuf.String())}
}

func TestOrgsFilterIsCaseInsensitiveSubstring(t *testing.T) {
	r := runWith(t, newFake(), nil, "orgs", "ACME")
	if r.code != 0 || r.stdout != "acme\nAcme-Labs\n" {
		t.Errorf("exit %d stdout %q stderr %q", r.code, r.stdout, r.stderr)
	}
	if r := runWith(t, newFake(), nil, "orgs"); r.stdout != "acme\nAcme-Labs\nother\n" {
		t.Errorf("unfiltered stdout %q", r.stdout)
	}
}

func TestModsLatestPrintsOneLinePerModuleWithDomainHost(t *testing.T) {
	r := runWith(t, newFake(), nil, "mods", "net")
	if r.code != 0 {
		t.Fatalf("exit %d stderr %q", r.code, r.stderr)
	}
	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2:\n%s", len(lines), r.stdout)
	}
	if !strings.HasPrefix(lines[0], "tfe.example.com/acme/network/azurerm") || !strings.Contains(lines[0], " 1.10.0 ") || !strings.Contains(lines[0], "2026-Feb-03 04:05") {
		t.Errorf("network line = %q, want the 1.10.0 latest with the domain host", lines[0])
	}
	if !strings.HasPrefix(lines[1], "tfe.example.com/hashicorp/netapp/azurerm") || !strings.Contains(lines[1], " 0.1.0 ") {
		t.Errorf("netapp line = %q", lines[1])
	}
}

func TestModsAllPrintsEveryVersionAcrossPages(t *testing.T) {
	r := runWith(t, newFake(), nil, "mods", "--all")
	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if r.code != 0 || len(lines) != 5 {
		t.Fatalf("exit %d lines %d:\n%s", r.code, len(lines), r.stdout)
	}
	if !strings.Contains(r.stdout, "acme/database/azurerm") {
		t.Errorf("second page missing from output:\n%s", r.stdout)
	}
}

func TestModsSingleMatchPrintsDetail(t *testing.T) {
	r := runWith(t, newFake(), nil, "mods", "database")
	want := []string{"# Terraform Cloud Registry Module", "id: mod-db", "name: database", "provider: azurerm", "namespace: acme",
		"created_at: 2026-Mar-04 05:06", "updated_at: 2026-Apr-05 06:07", "versions:", "modver-5", "2.0.0"}
	for _, s := range want {
		if !strings.Contains(r.stdout, s) {
			t.Errorf("detail lacks %q:\n%s", s, r.stdout)
		}
	}
	if r := runWith(t, newFake(), nil, "mods", "network"); !strings.Contains(r.stdout, "repo_url: https://github.com/acme/terraform-azurerm-network") {
		t.Errorf("repo_url missing:\n%s", r.stdout)
	}
}

func TestModsJSONPrintsSingleMatch(t *testing.T) {
	r := runWith(t, newFake(), nil, "mods", "-j", "database")
	if r.code != 0 {
		t.Fatalf("exit %d stderr %q", r.code, r.stderr)
	}
	var back tfe.RegistryModule
	if err := json.Unmarshal([]byte(r.stdout), &back); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, r.stdout)
	}
	if back.ID != "mod-db" || back.Name != "database" || len(back.VersionStatuses) != 1 {
		t.Errorf("decoded module = %+v", back)
	}
}

func TestModsNoMatchPrintsNothing(t *testing.T) {
	if r := runWith(t, newFake(), nil, "mods", "zzz"); r.code != 0 || r.stdout != "" {
		t.Errorf("exit %d stdout %q", r.code, r.stdout)
	}
}

func TestWsListFiltersAcrossPages(t *testing.T) {
	r := runWith(t, newFake(), nil, "ws", "network")
	if r.code != 0 || r.stdout != "prod-network\ndev-network\n" {
		t.Errorf("exit %d stdout %q", r.code, r.stdout)
	}
}

func TestShowPrintsAgentPoolAndVariableGroups(t *testing.T) {
	r := runWith(t, newFake(), nil, "show", "prod-network")
	if r.code != 0 {
		t.Fatalf("exit %d stderr %q", r.code, r.stderr)
	}
	want := "# Terraform Cloud Workspace\n" +
		"workspace_name: prod-network\n" +
		"workspace_id: ws-1\n" +
		"created_at: 2026-Sep-08 09:30\n" +
		"updated_at: 2026-Sep-08 10:30\n" +
		"description: Production network\n" +
		"terraform_version: 1.9.5\n" +
		"auto_apply: true\n" +
		"working_directory: \"\"\n" +
		"execution_mode: agent\n" +
		"  agent_pool_name: on-prem-agents\n" +
		"variables:\n" +
		"  environment:\n" +
		"    ARM_CLIENT_SECRET: \n" +
		"  terraform:\n" +
		"    region: eastus\n" +
		"    tags: {team=\"net\"}\n"
	if r.stdout != want {
		t.Errorf("show =\n%s\nwant\n%s", r.stdout, want)
	}
	if r := runWith(t, newFake(), nil, "show", "staging-db"); strings.Contains(r.stdout, "agent_pool") || strings.Contains(r.stdout, "environment:") {
		t.Errorf("remote workspace without variables printed extra sections:\n%s", r.stdout)
	}
}

func TestCloneCopiesSettingsAndVariablesAndWarnsOnSensitive(t *testing.T) {
	f := newFake()
	r := runWith(t, f, nil, "clone", "prod-network", "prod-network-copy")
	if r.code != 0 {
		t.Fatalf("exit %d stderr %q", r.code, r.stderr)
	}
	if len(f.created) != 1 {
		t.Fatalf("workspaces created = %d, want 1", len(f.created))
	}
	c := f.created[0]
	if *c.Name != "prod-network-copy" || !*c.AutoApply || *c.TerraformVersion != "1.9.5" || *c.WorkingDirectory != "" ||
		*c.Description != "Production network" || *c.ExecutionMode != "agent" || c.AgentPoolID == nil || *c.AgentPoolID != "apool-1" {
		t.Errorf("create options = %+v", c)
	}
	vars := f.createdVars["ws-new"]
	if len(vars) != 3 {
		t.Fatalf("variables created = %d, want 3", len(vars))
	}
	got := fmt.Sprintf("%s|%s|%s|%t|%t", *vars[0].Key, *vars[0].Value, *vars[0].Category, *vars[0].HCL, *vars[0].Sensitive)
	if got != "ARM_CLIENT_SECRET||env|false|true" {
		t.Errorf("first variable = %s", got)
	}
	if *vars[2].Key != "tags" || !*vars[2].HCL || *vars[2].Category != tfe.CategoryTerraform {
		t.Errorf("third variable = %+v", vars[2])
	}
	if !strings.Contains(r.stdout, "Cloned workspace prod-network to prod-network-copy") ||
		!strings.Contains(r.stdout, "Sensitive variables were copied with empty values; set them again in prod-network-copy: ARM_CLIENT_SECRET") {
		t.Errorf("stdout = %q", r.stdout)
	}
	if r := runWith(t, newFake(), nil, "clone", "staging-db", "staging-db-copy"); strings.Contains(r.stdout, "Sensitive") {
		t.Errorf("clone without sensitive variables still warned:\n%s", r.stdout)
	}
}

func TestEveryAPIFailureExitsOneNamingTheOperation(t *testing.T) {
	cases := []struct {
		failOp string
		args   []string
		want   string
	}{
		{"orgs", []string{"orgs"}, "listing organizations"},
		{"mods", []string{"mods"}, "listing modules for organization acme"},
		{"ws", []string{"ws"}, "listing workspaces for organization acme"},
		{"read", []string{"show", "prod-network"}, "reading workspace prod-network in organization acme"},
		{"vars", []string{"show", "prod-network"}, "listing variables for workspace prod-network"},
		{"read", []string{"clone", "prod-network", "x"}, "reading source workspace prod-network in organization acme"},
		{"create", []string{"clone", "prod-network", "x"}, "creating workspace x in organization acme"},
		{"createvar", []string{"clone", "prod-network", "x"}, "creating variable ARM_CLIENT_SECRET in workspace x"},
	}
	for _, c := range cases {
		f := newFake()
		f.failOp = c.failOp
		r := runWith(t, f, nil, c.args...)
		if r.code != 1 || !strings.Contains(r.stderr, "tfe: "+c.want) || !strings.Contains(r.stderr, "boom") {
			t.Errorf("%s %v: exit %d stderr %q, want exit 1 with %q", c.failOp, c.args, r.code, r.stderr, c.want)
		}
	}
	if r := runWith(t, newFake(), nil, "show", "missing"); r.code != 1 || !strings.Contains(r.stderr, "reading workspace missing") {
		t.Errorf("missing workspace: exit %d stderr %q", r.code, r.stderr)
	}
}

func TestCompareVersionsOrdersNumerically(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.9.0", "1.10.0", -1}, {"1.10.0", "1.9.0", 1}, {"2.0.0", "2.0.0", 0}, {"v1.2", "1.2.0", 0}, {"1.2.1", "1.2", 1}, {"1.0.0-rc1", "1.0.0", -1},
	}
	for _, c := range cases {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestRegistryHostUsesTheConfiguredDomain(t *testing.T) {
	cases := map[string]string{"https://app.terraform.io": "app.terraform.io", "https://tfe.example.com/": "tfe.example.com", "tfe.example.com": "tfe.example.com"}
	for in, want := range cases {
		if got := registryHost(in); got != want {
			t.Errorf("registryHost(%q) = %q, want %q", in, got, want)
		}
	}
}
