package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAliasesAndForms(t *testing.T) {
	tests := []struct {
		args     []string
		name     string
		operands []string
	}{
		{nil, "help", nil}, {[]string{"s"}, "status", nil}, {[]string{"pull", "one"}, "pull", []string{"one"}},
		{[]string{"b", "one"}, "build", []string{"one"}}, {[]string{"l"}, "list", nil}, {[]string{"c", "owner/repo"}, "clone", []string{"owner/repo"}},
	}
	for _, test := range tests {
		got, err := parse(test.args)
		if err != nil {
			t.Fatalf("parse(%v): %v", test.args, err)
		}
		if got.name != test.name || strings.Join(got.args, ",") != strings.Join(test.operands, ",") {
			t.Errorf("parse(%v) = %#v", test.args, got)
		}
	}
	if _, err := parse([]string{"list", "one"}); err == nil {
		t.Error("list accepted an operand")
	}
	if _, err := parse([]string{"wat"}); err == nil {
		t.Error("unknown command accepted")
	}
}

func TestRunHelpAndVersion(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-?"}, {"--version"}} {
		var out bytes.Buffer
		if err := run(args, &out, &bytes.Buffer{}); err != nil {
			t.Fatalf("run(%v): %v", args, err)
		}
		if !strings.Contains(out.String(), "repoctl") {
			t.Errorf("run(%v) output = %q", args, out.String())
		}
	}
}

func TestDiscoveryAndStatus(t *testing.T) {
	dir := t.TempDir()
	old, _ := os.Getwd()
	defer os.Chdir(old)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "clean", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".hidden", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	repos, err := discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 1 || repos[0].name != "clean" {
		t.Fatalf("repos = %#v", repos)
	}
	if !routinePull("Already up to date.\n") || routinePull("Already up to date\nextra") {
		t.Error("routine pull detection failed")
	}
}

func TestOutputHelpers(t *testing.T) {
	repos := []repoResult{{name: "b", origin: "https://b/x.git"}, {name: "long", origin: "<no origin>"}}
	w := widths(repos)
	text := resultRow(repoResult{name: "b", origin: repos[0].origin, status: "👍 main", details: []string{"detail"}}, false, w)
	if !strings.Contains(text, "https://b/x") || strings.Contains(text, ".git") || !strings.Contains(text, "    detail") {
		t.Errorf("row = %q", text)
	}
}

func TestValidationHelpers(t *testing.T) {
	for _, name := range []string{"repo", "repo-name"} {
		if !validName(name) {
			t.Errorf("validName(%q) false", name)
		}
	}
	for _, name := range []string{"", ".", "..", "a/b", `a\\b`} {
		if validName(name) {
			t.Errorf("validName(%q) true", name)
		}
	}
	if !validQualified("owner/repo") || validQualified("owner/repo/extra") || validQualified("/repo") {
		t.Error("qualified-name validation failed")
	}
}

func TestStatusPullBuildAndCloneFixtures(t *testing.T) {
	dir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "repo", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeExecutable(t, dir, "git", `#!/bin/sh
case "$1:$2:$3" in
  remote:get-url:origin) echo https://example.test/repo.git ;;
  branch:--show-current:) echo main ;;
  status:--porcelain:) ;;
  ls-remote::) ;;
  pull::) echo "Already up to date." ;;
  clone:*) mkdir -p "$3/.git" ;;
  *) exit 9 ;;
esac
`)
	withPath(t, dir)
	status := statusRepo(repoResult{name: "repo", origin: "https://example.test/repo.git"})
	if status.failed || status.status != "👍 main" {
		t.Fatalf("status = %#v", status)
	}
	var subsetOutput bytes.Buffer
	if err := runLocal("status", []string{"repo"}, false, &subsetOutput); err != nil {
		t.Fatalf("status subset: %v", err)
	}
	if !strings.Contains(subsetOutput.String(), "repo") {
		t.Errorf("subset output = %q", subsetOutput.String())
	}
	pull := pullRepo(repoResult{name: "repo", origin: status.origin})
	if pull.failed || pull.status != "Already up to date" {
		t.Fatalf("pull = %#v", pull)
	}
	buildScript := filepath.Join(dir, "repo", "build.sh")
	if err := os.WriteFile(buildScript, []byte("#!/bin/sh\necho build-ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if built := buildRepo(repoResult{name: "repo"}, false); built.failed || built.status != "Built" {
		t.Fatalf("build = %#v", built)
	}
	forcePath := filepath.Join(dir, "force-tty")
	if err := os.WriteFile(buildScript, []byte("#!/bin/sh\nprintf '%s' \"$GOVERNA_FORCE_TTY\" > "+forcePath+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldForce, hadForce := os.LookupEnv("GOVERNA_FORCE_TTY")
	if err := os.Unsetenv("GOVERNA_FORCE_TTY"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if hadForce {
			_ = os.Setenv("GOVERNA_FORCE_TTY", oldForce)
		} else {
			_ = os.Unsetenv("GOVERNA_FORCE_TTY")
		}
	}()
	if built := buildRepo(repoResult{name: "repo"}, true); built.failed || built.status != "Built" {
		t.Fatalf("colored build = %#v", built)
	}
	force, err := os.ReadFile(forcePath)
	if err != nil || string(force) != "1" {
		t.Errorf("GOVERNA_FORCE_TTY = %q, err=%v", force, err)
	}
	var cloneOutput bytes.Buffer
	if err := runClone([]string{"owner", "newrepo"}, false, &cloneOutput); err != nil {
		t.Fatalf("clone: %v", err)
	}
	if !strings.Contains(cloneOutput.String(), "Cloned") || !strings.Contains(cloneOutput.String(), "owner/newrepo") {
		t.Errorf("clone output = %q", cloneOutput.String())
	}
}

func TestListUsesScopedGitHubOwners(t *testing.T) {
	dir := t.TempDir()
	writeExecutable(t, dir, "gh", `#!/bin/sh
case "$1:$2:$3" in
  api:user:*) echo account ;;
  api:user/orgs:*) echo org ;;
  repo:list:account) echo alpha ;;
  repo:list:org) echo beta ;;
  *) exit 9 ;;
esac
`)
	withPath(t, dir)
	var output bytes.Buffer
	if err := runList(&output); err != nil {
		t.Fatalf("list: %v", err)
	}
	if got := output.String(); got != "account/alpha\norg/beta\n" {
		t.Errorf("list output = %q", got)
	}
}

func TestScopedCloneResolution(t *testing.T) {
	dir := t.TempDir()
	writeExecutable(t, dir, "gh", `#!/bin/sh
case "$1:$2:$3" in
  api:user:*) echo account ;;
  api:user/orgs:*) echo org ;;
  repo:list:account) echo unique ; echo shared ;;
  repo:list:org) echo other ; echo shared ;;
  *) exit 9 ;;
esac
`)
	withPath(t, dir)
	owner, names, err := resolveClone([]string{"unique"})
	if err != nil || owner != "account" || len(names) != 1 || names[0] != "unique" {
		t.Fatalf("single scoped match = %q %#v, err=%v", owner, names, err)
	}
	if _, _, err := resolveClone([]string{"missing"}); err == nil {
		t.Error("zero scoped matches did not fail")
	}
	if _, _, err := resolveClone([]string{"shared"}); err == nil {
		t.Error("ambiguous scoped matches did not fail")
	}
}

func TestPullContinuesAfterRepositoryFailure(t *testing.T) {
	dir := t.TempDir()
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bad", "good"} {
		if err := os.MkdirAll(filepath.Join(dir, name, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeExecutable(t, dir, "git", `#!/bin/sh
repo=${PWD##*/}
case "$1:$2:$3" in
  remote:get-url:origin) echo "https://example.test/$repo.git" ;;
  ls-remote::) ;;
  pull::) [ "$repo" = bad ] && { echo "pull failed" >&2; exit 8; }; echo "updated" ;;
  *) exit 9 ;;
esac
`)
	withPath(t, dir)
	var output bytes.Buffer
	if err := runLocal("pull", nil, false, &output); err == nil {
		t.Fatal("pull returned success after a repository failure")
	}
	text := output.String()
	if !strings.Contains(text, "bad") || !strings.Contains(text, "Pull failed") || !strings.Contains(text, "good") || !strings.Contains(text, "Pulled") {
		t.Errorf("pull output = %q", text)
	}
}

func writeExecutable(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func withPath(t *testing.T, dir string) {
	t.Helper()
	old := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", old) })
	os.Setenv("PATH", dir+string(os.PathListSeparator)+old)
}
