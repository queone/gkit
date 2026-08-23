package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
  repo:list:account) printf '%s\n' '[{"nameWithOwner":"account/alpha","description":"Alpha CLI","visibility":"PUBLIC","isArchived":false,"updatedAt":"2026-08-22T11:54:00Z"}]' ;;
  repo:list:org) printf '%s\n' '[{"nameWithOwner":"org/gamma","description":"","visibility":"PRIVATE","isArchived":false,"updatedAt":"2026-08-22T11:54:00Z"},{"nameWithOwner":"org/beta","description":"  Internal   tools  ","visibility":"INTERNAL","isArchived":true,"updatedAt":"2026-08-21T12:00:00Z"}]' ;;
  *) exit 9 ;;
esac
`)
	withPath(t, dir)
	var output bytes.Buffer
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	if err := runListAt(&output, now, false); err != nil {
		t.Fatalf("list: %v", err)
	}
	want := "REPO                      DESCRIPTION     INFO                UPDATED\n" +
		"github.com/account/alpha  Alpha CLI       public              about 6 minutes ago\n" +
		"github.com/org/beta       Internal tools  internal, archived  about 1 day ago\n" +
		"github.com/org/gamma                      private             about 6 minutes ago\n"
	if got := output.String(); got != want {
		t.Errorf("list output = %q", got)
	}
	var colored bytes.Buffer
	if err := runListAt(&colored, now, true); err != nil {
		t.Fatalf("colored list: %v", err)
	}
	gotColored := colored.String()
	if !strings.Contains(gotColored, yellow+"private"+reset) || !strings.Contains(gotColored, yellow+"archived"+reset) {
		t.Errorf("colored list lacks yellow INFO labels: %q", gotColored)
	}
	withoutANSI := strings.ReplaceAll(strings.ReplaceAll(gotColored, yellow, ""), reset, "")
	if withoutANSI != want {
		t.Errorf("colored list alignment differs after removing ANSI: %q", withoutANSI)
	}
}

func TestRemoteRepoSortOrder(t *testing.T) {
	older := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	repos := []remoteRepo{
		{NameWithOwner: "org/zeta", UpdatedAt: newer},
		{NameWithOwner: "org/alpha", UpdatedAt: older},
		{NameWithOwner: "org/alpha", UpdatedAt: newer},
	}
	sortRemoteRepos(repos)
	if repos[0].NameWithOwner != "org/alpha" || !repos[0].UpdatedAt.Equal(newer) ||
		repos[1].NameWithOwner != "org/alpha" || !repos[1].UpdatedAt.Equal(older) ||
		repos[2].NameWithOwner != "org/zeta" {
		t.Errorf("repository order = %#v", repos)
	}
}

func TestRepoInfoColorsPrivateAndArchived(t *testing.T) {
	tests := []struct {
		name string
		repo remoteRepo
		want string
	}{
		{"public", remoteRepo{Visibility: "PUBLIC"}, "public"},
		{"internal", remoteRepo{Visibility: "INTERNAL"}, "internal"},
		{"private", remoteRepo{Visibility: "PRIVATE"}, yellow + "private" + reset},
		{"public archived", remoteRepo{Visibility: "PUBLIC", IsArchived: true}, "public, " + yellow + "archived" + reset},
		{"private archived", remoteRepo{Visibility: "PRIVATE", IsArchived: true}, yellow + "private" + reset + ", " + yellow + "archived" + reset},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := repoInfo(test.repo, true); got != test.want {
				t.Errorf("repository info = %q, want %q", got, test.want)
			}
		})
	}
	if got := repoInfo(remoteRepo{Visibility: "PRIVATE", IsArchived: true}, false); got != "private, archived" {
		t.Errorf("plain repository info = %q", got)
	}
}

func TestListColorDisableControls(t *testing.T) {
	var output bytes.Buffer
	if colorEnabled(&output) {
		t.Error("non-TTY output enabled color")
	}
	t.Setenv("NO_COLOR", "1")
	if colorEnabled(os.Stdout) {
		t.Error("NO_COLOR enabled color")
	}
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "dumb")
	if colorEnabled(os.Stdout) {
		t.Error("TERM=dumb enabled color")
	}
}

func TestRelativeAgeUnits(t *testing.T) {
	now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		age  time.Duration
		want string
	}{
		{"future", -time.Second, "about 0 seconds ago"},
		{"second", time.Second, "about 1 second ago"},
		{"seconds", 59*time.Second + 999*time.Millisecond, "about 59 seconds ago"},
		{"minute", time.Minute, "about 1 minute ago"},
		{"minutes", 59*time.Minute + 59*time.Second, "about 59 minutes ago"},
		{"hour", time.Hour, "about 1 hour ago"},
		{"hours", 23*time.Hour + 59*time.Minute, "about 23 hours ago"},
		{"day", 24 * time.Hour, "about 1 day ago"},
		{"days", 29*24*time.Hour + 23*time.Hour, "about 29 days ago"},
		{"month", 30 * 24 * time.Hour, "about 1 month ago"},
		{"months", 364 * 24 * time.Hour, "about 12 months ago"},
		{"year", 365 * 24 * time.Hour, "about 1 year ago"},
		{"years", 2 * 365 * 24 * time.Hour, "about 2 years ago"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := relativeAge(now.Add(-test.age), now); got != test.want {
				t.Errorf("relativeAge() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRemoteRepoDetailsRequestsOneThousandRepositories(t *testing.T) {
	dir := t.TempDir()
	writeExecutable(t, dir, "gh", `#!/bin/sh
[ "$1:$2:$3:$4:$5:$6:$7" = "repo:list:account:--limit:1000:--json:nameWithOwner,description,visibility,isArchived,updatedAt" ] || exit 9
printf '['
i=1
while [ "$i" -le 31 ]; do
  [ "$i" -eq 1 ] || printf ','
  printf '{"nameWithOwner":"account/repo%s","description":"","visibility":"PRIVATE","isArchived":false,"updatedAt":"2026-08-22T12:00:00Z"}' "$i"
  i=$((i + 1))
done
printf ']\n'
`)
	withPath(t, dir)
	repos, err := remoteRepoDetails("account")
	if err != nil {
		t.Fatalf("repository details: %v", err)
	}
	if len(repos) != 31 {
		t.Fatalf("repository count = %d, want 31", len(repos))
	}
}

func TestListContinuesAfterOwnerFailure(t *testing.T) {
	dir := t.TempDir()
	writeExecutable(t, dir, "gh", `#!/bin/sh
case "$1:$2:$3" in
  api:user:*) echo account ;;
  api:user/orgs:*) echo org ;;
  repo:list:account) printf '%s\n' '[{"nameWithOwner":"account/alpha","description":"","visibility":"PRIVATE","isArchived":false,"updatedAt":"2026-08-22T12:00:00Z"}]' ;;
  repo:list:org) exit 8 ;;
  *) exit 9 ;;
esac
`)
	withPath(t, dir)
	var output bytes.Buffer
	err := runListAt(&output, time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC), false)
	if err == nil {
		t.Fatal("list returned success after an owner failure")
	}
	got := output.String()
	if !strings.Contains(got, "run gh repo list org") || !strings.Contains(got, "github.com/account/alpha") {
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
