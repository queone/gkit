package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/queone/gkit/internal/color"
)

type runResult struct {
	code   int
	stdout string
	stderr string
	sleeps []time.Duration
}

// runWith runs the utility with captured output, a recording sleep, and the given sites file path.
// An empty sitesFile means a missing file.
func runWith(t *testing.T, sitesFile string, client *http.Client, stdin string, args ...string) runResult {
	t.Helper()
	if sitesFile == "" {
		sitesFile = filepath.Join(t.TempDir(), "sites")
	}
	if client == nil {
		client = &http.Client{Timeout: time.Second}
	}
	var out, errBuf bytes.Buffer
	var sleeps []time.Duration
	code := run(args, env{
		stdin:     strings.NewReader(stdin),
		stdout:    &out,
		stderr:    &errBuf,
		sitesPath: sitesFile,
		client:    client,
		sleep:     func(d time.Duration) { sleeps = append(sleeps, d) },
	})
	return runResult{code: code, stdout: color.ClearCode(out.String()), stderr: errBuf.String(), sleeps: sleeps}
}

// writeSites writes a sites file in a temporary directory and returns its path.
func writeSites(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "sites")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// newSiteServer serves a fake profile site whose answer depends on the username in the path.
func newSiteServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits, flaky atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch strings.TrimPrefix(r.URL.Path, "/") {
		case "taken":
			w.WriteHeader(http.StatusOK)
		case "forbidden":
			w.WriteHeader(http.StatusForbidden)
		case "gone":
			w.WriteHeader(http.StatusGone)
		case "flaky":
			if flaky.Add(1) == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		case "nohead":
			if r.Method == http.MethodHead {
				w.WriteHeader(http.StatusMethodNotAllowed)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		case "limited":
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestPatternClassesExpandToEveryCombination(t *testing.T) {
	got, err := expandPattern("[qk][aeou][qk][aeou]")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 64 || got[0] != "qaqa" || got[63] != "kuku" {
		t.Errorf("4-class pattern = %d names, first %q, last %q; want 64, qaqa, kuku", len(got), got[0], got[len(got)-1])
	}
	cases := map[string][]string{
		"a[bc]d": {"abd", "acd"},
		"[aa]":   {"a"},
		"kaqe":   {"kaqe"},
	}
	for p, want := range cases {
		got, err := expandPattern(p)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("expandPattern(%q) = %q, %v; want %q", p, got, err, want)
		}
	}
}

func TestMalformedPatternIsRejected(t *testing.T) {
	for _, p := range []string{"[qk", "[]", "ab]", "[qk][", "]"} {
		if _, err := expandPattern(p); err == nil {
			t.Errorf("expandPattern(%q) accepted a malformed pattern", p)
		}
	}
}

func TestRetiredScansReproduceCandidateCounts(t *testing.T) {
	cases := []struct {
		args []string
		want int
	}{
		{[]string{"-n", "github", "[aeouqktpm][aeouqktpm][aeouqktpm]"}, 729},
		{[]string{"-n", "-d", "-r", "qk", "github", "[qkaeou][qkaeou][qkaeou][qkaeou]"}, 144},
		{[]string{"-n", "-d", "lichess", "[qk][aeiou][qk][aeiou]"}, 40},
	}
	for _, c := range cases {
		r := runWith(t, "", nil, "", c.args...)
		if r.code != 0 {
			t.Errorf("%q exit = %d, stderr %q", c.args, r.code, r.stderr)
		}
		if got := strings.Count(r.stdout, "\n"); got != c.want {
			t.Errorf("%q printed %d names, want %d", c.args, got, c.want)
		}
	}
}

func TestDryRunPrintsCandidatesWithoutRequests(t *testing.T) {
	srv, hits := newSiteServer(t)
	r := runWith(t, "", srv.Client(), "", "-n", srv.URL+"/{}", "kaqe")
	if r.code != 0 || r.stdout != "kaqe\n" {
		t.Errorf("dry run = exit %d, stdout %q; want 0, kaqe", r.code, r.stdout)
	}
	if hits.Load() != 0 {
		t.Errorf("dry run sent %d request(s)", hits.Load())
	}
	if r := runWith(t, "", nil, "", "-n", "github", "kaqe"); r.stdout != "kaqe\n" {
		t.Errorf("built-in dry run stdout = %q, want kaqe", r.stdout)
	}
}

func TestFiltersKeepDistinctAndRequiredCharacters(t *testing.T) {
	cases := []struct {
		name     string
		distinct bool
		require  string
		want     bool
	}{
		{"kaqe", true, "", true},
		{"kaak", true, "", false},
		{"kaqe", false, "qk", true},
		{"kate", false, "qk", false},
		{"kaqe", true, "qk", true},
		{"kaak", false, "", true},
	}
	for _, c := range cases {
		if got := keep(c.name, c.distinct, c.require); got != c.want {
			t.Errorf("keep(%q, %v, %q) = %v, want %v", c.name, c.distinct, c.require, got, c.want)
		}
	}
}

func TestLiteralExcludedByFilterExitsTwo(t *testing.T) {
	r := runWith(t, "", nil, "", "-d", "github", "kaak")
	if r.code != 2 || !strings.Contains(r.stderr, "excluded") {
		t.Errorf("filtered literal = exit %d, stderr %q; want 2 and an excluded message", r.code, r.stderr)
	}
}

func TestStdinCandidatesSkipBlankLines(t *testing.T) {
	r := runWith(t, "", nil, "kaqe\n\n  qake \n", "-n", "github", "-")
	if r.code != 0 || r.stdout != "kaqe\nqake\n" {
		t.Errorf("stdin candidates = exit %d, stdout %q; want 0 and kaqe, qake", r.code, r.stdout)
	}
}

func TestSiteResolutionPrefersFileOverBuiltIn(t *testing.T) {
	p := writeSites(t, "reddit https://www.reddit.com/user/{}\ngithub https://example.test/{} 404,410\n")
	sites, err := loadSites(p)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range sites {
		names = append(names, s.name)
	}
	if want := []string{"archive", "github", "lichess", "reddit"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("site names = %q, want %q", names, want)
	}
	if sites[1].template != "https://example.test/{}" || sites[1].source != sourceFile || !reflect.DeepEqual(sites[1].free, []int{404, 410}) {
		t.Errorf("github override = %+v", sites[1])
	}
	if sites[0].source != sourceBuiltIn || sites[2].source != sourceBuiltIn || sites[3].source != sourceFile {
		t.Errorf("sources = %q, %q, %q; want built-in, built-in, file", sites[0].source, sites[2].source, sites[3].source)
	}

	s, err := resolveSite("https://x.test/u/{}", sites)
	if err != nil || s.source != sourceArgument || s.template != "https://x.test/u/{}" {
		t.Errorf("template argument = %+v, %v", s, err)
	}
	if _, err := resolveSite("nosuchsite", sites); err == nil || !strings.Contains(err.Error(), "known sites: archive, github, lichess, reddit") {
		t.Errorf("unknown site error = %v", err)
	}
	if _, err := resolveSite("https://x.test/u", sites); err == nil || !strings.Contains(err.Error(), "{}") {
		t.Errorf("template without {} error = %v", err)
	}
}

func TestTemplateMustBeAbsoluteHTTP(t *testing.T) {
	for _, bad := range []string{"example.com/{}", "ftp://example.com/{}", "https://example.com/u", "https:///{}", "{}"} {
		if err := validateTemplate(bad); err == nil {
			t.Errorf("validateTemplate(%q) accepted a bad template", bad)
		}
	}
	for _, good := range []string{"https://github.com/{}", "http://localhost:8080/u/{}"} {
		if err := validateTemplate(good); err != nil {
			t.Errorf("validateTemplate(%q) = %v", good, err)
		}
	}
}

func TestSitesFileParsing(t *testing.T) {
	sites, err := parseSites(strings.NewReader("# comment\n\n  reddit   https://www.reddit.com/user/{}   404,410  \nplain https://p.test/{}\n"), "sites")
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 2 || !reflect.DeepEqual(sites[0].free, []int{404, 410}) || sites[1].free != nil {
		t.Errorf("parsed sites = %+v", sites)
	}

	bad := map[string]string{
		"one field on line 3":        "a https://a.test/{}\n\nlonely\n",
		"second code list on line 2": "a https://a.test/{}\nb https://b.test/{} 404 extra\n",
		"relative template line 1":   "a a.test/{}\n",
		"bad codes on line 2":        "a https://a.test/{}\nb https://b.test/{} abc\n",
	}
	wantLine := map[string]string{
		"one field on line 3":        "line 3",
		"second code list on line 2": "line 2",
		"relative template line 1":   "line 1",
		"bad codes on line 2":        "line 2",
	}
	for name, content := range bad {
		_, err := parseSites(strings.NewReader(content), "sites")
		if err == nil || !strings.Contains(err.Error(), wantLine[name]) || !strings.HasPrefix(err.Error(), "sites ") {
			t.Errorf("%s: error = %v, want it to name sites and %s", name, err, wantLine[name])
		}
	}

	sites, err = loadSites(filepath.Join(t.TempDir(), "missing"))
	if err != nil || len(sites) != 3 {
		t.Errorf("missing file = %d sites, %v; want the 3 built-ins", len(sites), err)
	}
}

func TestFreeCodePrecedence(t *testing.T) {
	srv, _ := newSiteServer(t)
	p := writeSites(t, "mysite "+srv.URL+"/{} 410\n")
	if r := runWith(t, p, srv.Client(), "", "mysite", "gone"); r.code != 0 || r.stdout != "gone: free "+srv.URL+"/gone\n" {
		t.Errorf("file codes = exit %d, stdout %q; want 0, free", r.code, r.stdout)
	}
	if r := runWith(t, p, srv.Client(), "", "-f", "404", "mysite", "gone"); r.code != 2 || r.stdout != "gone: unknown (HTTP 410)\n" {
		t.Errorf("flag overrides file = exit %d, stdout %q; want 2, unknown", r.code, r.stdout)
	}
	if r := runWith(t, "", srv.Client(), "", srv.URL+"/{}", "gone"); r.code != 2 || r.stdout != "gone: unknown (HTTP 410)\n" {
		t.Errorf("default codes = exit %d, stdout %q; want 2, unknown", r.code, r.stdout)
	}
}

func TestClassificationOutcomes(t *testing.T) {
	srv, _ := newSiteServer(t)
	tmpl := srv.URL + "/{}"
	cases := []struct {
		name   string
		code   int
		stdout string
		sleeps []time.Duration
	}{
		{"free", 0, "free: free " + srv.URL + "/free\n", nil},
		{"taken", 1, "taken: taken\n", nil},
		{"forbidden", 2, "forbidden: unknown (HTTP 403)\n", nil},
		{"flaky", 0, "flaky: free " + srv.URL + "/flaky\n", []time.Duration{100 * time.Millisecond}},
		{"nohead", 0, "nohead: free " + srv.URL + "/nohead\n", nil},
		{"limited", 2, "limited: unknown (HTTP 429 after 5 attempts)\n", []time.Duration{100 * time.Millisecond, 200 * time.Millisecond, 400 * time.Millisecond, 800 * time.Millisecond}},
	}
	for _, c := range cases {
		r := runWith(t, "", srv.Client(), "", tmpl, c.name)
		if r.code != c.code || r.stdout != c.stdout {
			t.Errorf("%s = exit %d, stdout %q; want %d, %q", c.name, r.code, r.stdout, c.code, c.stdout)
		}
		if !reflect.DeepEqual(r.sleeps, c.sleeps) {
			t.Errorf("%s sleeps = %v, want %v", c.name, r.sleeps, c.sleeps)
		}
	}
}

func TestTransportErrorIsUnknownAfterRetries(t *testing.T) {
	dead := httptest.NewServer(http.NotFoundHandler())
	url := dead.URL
	dead.Close()
	r := runWith(t, "", nil, "", url+"/{}", "kaqe")
	if r.code != 2 || !strings.HasPrefix(r.stdout, "kaqe: unknown (") || !strings.Contains(r.stdout, "after 5 attempts)") {
		t.Errorf("dead server = exit %d, stdout %q", r.code, r.stdout)
	}
	if len(r.sleeps) != 4 {
		t.Errorf("dead server slept %d times, want 4", len(r.sleeps))
	}
}

func TestScanPrintsFreeNamesAndSummary(t *testing.T) {
	srv, _ := newSiteServer(t)
	tmpl := srv.URL + "/{}"
	r := runWith(t, "", srv.Client(), "free\ntaken\nforbidden\n", tmpl, "-")
	if r.code != 2 || r.stdout != "free "+srv.URL+"/free\n" {
		t.Errorf("mixed scan = exit %d, stdout %q; want 2, free with its URL", r.code, r.stdout)
	}
	if !strings.Contains(r.stderr, "forbidden: unknown (HTTP 403)\n") || !strings.HasSuffix(r.stderr, "Checked 3: 1 free, 1 taken, 1 unknown.\n"+signupNote+"\n") {
		t.Errorf("mixed scan stderr = %q", r.stderr)
	}
	r = runWith(t, "", srv.Client(), "free\ntaken\n", tmpl, "-")
	if r.code != 0 || r.stdout != "free "+srv.URL+"/free\n" || r.stderr != "Checked 2: 1 free, 1 taken, 0 unknown.\n"+signupNote+"\n" {
		t.Errorf("clean scan = exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}
}

func TestPauseBetweenRequestsDefaultsTo300ms(t *testing.T) {
	srv, _ := newSiteServer(t)
	tmpl := srv.URL + "/{}"
	cases := []struct {
		args []string
		want []time.Duration
	}{
		{[]string{tmpl, "-"}, []time.Duration{300 * time.Millisecond, 300 * time.Millisecond}},
		{[]string{"-w", "0", tmpl, "-"}, nil},
		{[]string{"-w", "50", tmpl, "-"}, []time.Duration{50 * time.Millisecond, 50 * time.Millisecond}},
	}
	for _, c := range cases {
		r := runWith(t, "", srv.Client(), "a\nb\nc\n", c.args...)
		if r.code != 0 || !reflect.DeepEqual(r.sleeps, c.want) {
			t.Errorf("%q = exit %d, sleeps %v; want 0, %v", c.args, r.code, r.sleeps, c.want)
		}
	}
	if r := runWith(t, "", srv.Client(), "", tmpl, "a"); len(r.sleeps) != 0 {
		t.Errorf("single name slept %v; want no pacing sleep", r.sleeps)
	}
}

func TestBadInputExitsTwoWithoutRequests(t *testing.T) {
	srv, hits := newSiteServer(t)
	cases := [][]string{
		{"nosuchsite", "kaqe"},
		{"example.com/{}", "kaqe"},
		{"https://example.com/u", "kaqe"},
		{"github", "[qk"},
		{"-f", "abc", "github", "kaqe"},
		{"github"},
		{"-l", "github"},
		{"-w", "-5", "github", "kaqe"},
		{"-r", "", "github", "kaqe"},
		{"-x", "github", "kaqe"},
		{"-r"},
	}
	for _, args := range cases {
		r := runWith(t, "", srv.Client(), "", args...)
		if r.code != 2 || r.stdout != "" || strings.Count(r.stderr, "\n") != 1 || !strings.HasPrefix(r.stderr, programName+": ") {
			t.Errorf("%q = exit %d, stdout %q, stderr %q; want 2 and one stderr line", args, r.code, r.stdout, r.stderr)
		}
	}
	if hits.Load() != 0 {
		t.Errorf("bad input sent %d request(s)", hits.Load())
	}
}

func TestListShowsFileAndBuiltInSites(t *testing.T) {
	p := writeSites(t, "reddit https://www.reddit.com/user/{}\ngithub https://example.test/{}\n")
	r := runWith(t, p, nil, "", "-l")
	if r.code != 0 {
		t.Fatalf("-l exit = %d, stderr %q", r.code, r.stderr)
	}
	lines := strings.Split(strings.TrimRight(r.stdout, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("-l printed %d lines: %q", len(lines), r.stdout)
	}
	want := []struct{ name, template, codes, source, profile string }{
		{"archive", "https://archive.org/download/@{}", "404", "built-in", "https://archive.org/details/@{}"},
		{"github", "https://example.test/{}", "404", "file", "https://example.test/{}"},
		{"lichess", "https://lichess.org/@/{}", "404", "built-in", "https://lichess.org/@/{}"},
		{"reddit", "https://www.reddit.com/user/{}", "404", "file", "https://www.reddit.com/user/{}"},
	}
	for i, w := range want {
		fields := strings.Fields(lines[i])
		got := strings.Join(fields, " ")
		if got != w.name+" "+w.template+" "+w.codes+" "+w.source+" "+w.profile {
			t.Errorf("-l line %d = %q, want %q", i+1, lines[i], w)
		}
	}

	r = runWith(t, "", nil, "", "-l")
	if r.code != 0 || strings.Count(r.stdout, "\n") != 3 || strings.Count(r.stdout, " file ") != 3 {
		t.Errorf("-l without a file = exit %d, stdout %q; want the three seeded built-ins as file", r.code, r.stdout)
	}

	p = writeSites(t, "reddit https://www.reddit.com/user/{}\n\nlonely\n")
	r = runWith(t, p, nil, "", "-l")
	if r.code != 2 || !strings.Contains(r.stderr, "line 3") {
		t.Errorf("malformed sites file = exit %d, stderr %q; want 2 naming line 3", r.code, r.stderr)
	}
}

func TestHelpOpensLikeOtherUtilitiesAndListsEveryFlag(t *testing.T) {
	flags := []string{"-d, --distinct", "-r, --require CHARS", "-n, --dry-run", "-f, --free CODES", "-w, --wait MS", "-l, --list", "-v, --version"}
	header := "namehunt v" + programVersion + "\nFind free usernames on any site with a predictable profile URL\ngithub.com/queone/gkit/tree/main/cmd/namehunt\n\nUsage\n  "
	bare := runWith(t, "", nil, "")
	if bare.code != 0 || !strings.HasPrefix(bare.stdout, header) {
		t.Errorf("bare namehunt = exit %d, stdout %q; want 0 and the vkeep-style header", bare.code, bare.stdout)
	}
	if !strings.Contains(bare.stdout, "\nUsage\n  namehunt [flags] SITE CANDIDATE") {
		t.Errorf("bare namehunt lacks the Usage block: %q", bare.stdout)
	}
	for _, f := range flags {
		if !strings.Contains(bare.stdout, "  "+f) {
			t.Errorf("help lacks flag line %q", f)
		}
	}
	for _, s := range []string{"Built-in sites: github, lichess, archive.", "(default 300)", "NAME TEMPLATE [FREE_CODES] [PROFILE_TEMPLATE]", "only signing up confirms"} {
		if !strings.Contains(bare.stdout, s) {
			t.Errorf("help lacks %q", s)
		}
	}
	for _, h := range []string{"-h", "-?", "--help"} {
		r := runWith(t, "", nil, "", h)
		if r.code != 0 || r.stdout != bare.stdout {
			t.Errorf("%s = exit %d; stdout differs from bare namehunt: %q", h, r.code, r.stdout)
		}
	}
	if r := runWith(t, "", nil, "", "-v"); r.code != 0 || r.stdout != "namehunt v"+programVersion+"\n" {
		t.Errorf("-v = exit %d, stdout %q", r.code, r.stdout)
	}
	if r := runWith(t, "", nil, "", "--version"); r.stdout != "namehunt v"+programVersion+"\n" {
		t.Errorf("--version stdout = %q", r.stdout)
	}
}

func TestSitesPathHonorsXDGConfigHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	p, err := sitesPath()
	if err != nil || p != filepath.Join(dir, "namehunt", "sites") {
		t.Errorf("absolute XDG_CONFIG_HOME path = %q, %v", p, err)
	}
	t.Setenv("XDG_CONFIG_HOME", "relative")
	p, err = sitesPath()
	if err != nil || !strings.HasSuffix(p, filepath.Join(".config", "namehunt", "sites")) {
		t.Errorf("relative XDG_CONFIG_HOME path = %q, %v; want the home fallback", p, err)
	}
}

func TestArchiveIsBuiltInWithDetailsProfile(t *testing.T) {
	sites, err := loadSites(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := resolveSite("archive", sites)
	if err != nil || s.source != sourceBuiltIn || s.template != "https://archive.org/download/@{}" || s.free != nil {
		t.Errorf("archive = %+v, %v; want the built-in download template with default codes", s, err)
	}
	if s.profileTemplate() != "https://archive.org/details/@{}" {
		t.Errorf("archive profile = %q, want the details URL", s.profileTemplate())
	}
	for _, name := range []string{"github", "lichess"} {
		s, err := resolveSite(name, sites)
		if err != nil || s.profileTemplate() != s.template {
			t.Errorf("%s profile = %q, %v; want its check template %q", name, s.profileTemplate(), err, s.template)
		}
	}
}

func TestMissingSitesFileIsSeeded(t *testing.T) {
	srv, _ := newSiteServer(t)
	for _, args := range [][]string{{"-l"}, {"-n", "github", "kaqe"}, {srv.URL + "/{}", "free"}} {
		p := filepath.Join(t.TempDir(), "cfg", "namehunt", "sites")
		if r := runWith(t, p, srv.Client(), "", args...); r.code != 0 {
			t.Fatalf("%q = exit %d, stderr %q", args, r.code, r.stderr)
		}
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%q did not seed %s: %v", args, p, err)
		}
	}
	for _, args := range [][]string{{"-h"}, {"-v"}} {
		p := filepath.Join(t.TempDir(), "cfg", "namehunt", "sites")
		runWith(t, p, nil, "", args...)
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%q seeded %s; want no file", args, p)
		}
	}

	p := filepath.Join(t.TempDir(), "cfg", "namehunt", "sites")
	first := runWith(t, p, nil, "", "-l")
	content, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != seedContent() {
		t.Errorf("seeded file = %q, want seedContent()", content)
	}
	var comments, siteLines []string
	for line := range strings.SplitSeq(strings.TrimRight(string(content), "\n"), "\n") {
		if strings.HasPrefix(line, "#") {
			comments = append(comments, line)
		} else {
			siteLines = append(siteLines, line)
		}
	}
	if len(comments) == 0 || len(siteLines) != 3 {
		t.Errorf("seeded file has %d comment and %d site lines; want some comments and 3 sites: %q", len(comments), len(siteLines), content)
	}
	if !strings.Contains(siteLines[2], "https://archive.org/download/@{}") || !strings.Contains(siteLines[2], "https://archive.org/details/@{}") {
		t.Errorf("archive line = %q, want both the download and details templates", siteLines[2])
	}
	parsed, err := parseSites(bytes.NewReader(content), p)
	if err != nil || len(parsed) != len(builtinSites) {
		t.Fatalf("seeded file parsed to %d sites, %v; want %d", len(parsed), err, len(builtinSites))
	}
	for i, b := range builtinSites {
		got := parsed[i]
		if got.name != b.name || got.template != b.template || !reflect.DeepEqual(got.free, b.free) || got.profile != b.profile {
			t.Errorf("seeded %s = %+v, want %+v", b.name, got, b)
		}
	}
	if first.code != 0 || strings.Count(first.stdout, "\n") != 3 || strings.Count(first.stdout, " file ") != 3 {
		t.Errorf("-l after seeding = exit %d, stdout %q; want three file entries", first.code, first.stdout)
	}
	second := runWith(t, p, nil, "", "-l")
	again, err := os.ReadFile(p)
	if err != nil || !bytes.Equal(content, again) || second.stdout != first.stdout || second.stderr != "" {
		t.Errorf("second run changed the file or output: %v, stderr %q", err, second.stderr)
	}
}

func TestSeedFailureWarnsAndContinues(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(blocker, "sites")
	r := runWith(t, p, nil, "", "-l")
	if r.code != 0 {
		t.Fatalf("-l with an unwritable sites path = exit %d, stderr %q", r.code, r.stderr)
	}
	if strings.Count(r.stderr, "\n") != 1 || !strings.HasPrefix(r.stderr, "namehunt: could not write the starter sites file "+p+": ") || !strings.HasSuffix(r.stderr, "; using the built-in sites\n") {
		t.Errorf("warning = %q", r.stderr)
	}
	if strings.Count(r.stdout, "\n") != 3 || strings.Count(r.stdout, " built-in ") != 3 {
		t.Errorf("-l after a failed seed = %q; want the three built-ins", r.stdout)
	}
}

func TestExistingSitesFileIsUntouched(t *testing.T) {
	srv, _ := newSiteServer(t)
	content := "mysite " + srv.URL + "/{}\n"
	p := writeSites(t, content)
	for _, args := range [][]string{{"-l"}, {"mysite", "taken"}} {
		runWith(t, p, srv.Client(), "", args...)
		got, err := os.ReadFile(p)
		if err != nil || string(got) != content {
			t.Errorf("%q changed the sites file: %q, %v", args, got, err)
		}
	}
	sites, err := loadSites(p)
	if err != nil {
		t.Fatal(err)
	}
	if s, err := resolveSite("archive", sites); err != nil || s.source != sourceBuiltIn || s.profile != "https://archive.org/details/@{}" {
		t.Errorf("archive with a file that omits it = %+v, %v; want the built-in", s, err)
	}
}

func TestSitesFileProfileFieldByShape(t *testing.T) {
	good := "a https://a.test/{} https://a.test/u/{}\nb https://b.test/{} 404,410 https://b.test/u/{}\nc https://c.test/{} https://c.test/u/{} 410\n"
	sites, err := parseSites(strings.NewReader(good), "sites")
	want := []site{
		{name: "a", template: "https://a.test/{}", profile: "https://a.test/u/{}", source: sourceFile},
		{name: "b", template: "https://b.test/{}", free: []int{404, 410}, profile: "https://b.test/u/{}", source: sourceFile},
		{name: "c", template: "https://c.test/{}", free: []int{410}, profile: "https://c.test/u/{}", source: sourceFile},
	}
	if err != nil || !reflect.DeepEqual(sites, want) {
		t.Errorf("parsed = %+v, %v; want %+v", sites, err, want)
	}

	bad := map[string]string{
		"profile without {} on line 1":     "a https://a.test/{} https://a.test/u\n",
		"profile without scheme on line 2": "a https://a.test/{}\nb https://b.test/{} ftp://b.test/{}\n",
		"five fields on line 1":            "a https://a.test/{} 404 https://a.test/u/{} extra\n",
		"two profiles on line 1":           "a https://a.test/{} https://a.test/u/{} https://a.test/v/{}\n",
		"two code lists on line 1":         "a https://a.test/{} 404 410\n",
	}
	for name, content := range bad {
		wantLine := name[strings.LastIndex(name, "line "):]
		_, err := parseSites(strings.NewReader(content), "sites")
		if err == nil || !strings.Contains(err.Error(), wantLine) || !strings.HasPrefix(err.Error(), "sites ") {
			t.Errorf("%s: error = %v, want it to name sites and %s", name, err, wantLine)
		}
	}

	p := writeSites(t, "github https://github.com/{} https://github.com/users/{}\n")
	sites, err = loadSites(p)
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := resolveSite("github", sites); s.source != sourceFile || s.profileTemplate() != "https://github.com/users/{}" {
		t.Errorf("github override = %+v; want the file's profile template", s)
	}
	if s, _ := resolveSite("lichess", sites); s.profileTemplate() != s.template {
		t.Errorf("lichess profile = %q; want its check template", s.profileTemplate())
	}
}

func TestFreeOutputShowsProfileURLAndSignupNote(t *testing.T) {
	srv, _ := newSiteServer(t)
	p := writeSites(t, "mysite "+srv.URL+"/{} https://p.test/u/{}\n")
	r := runWith(t, p, srv.Client(), "", "mysite", "free")
	if r.code != 0 || r.stdout != "free: free https://p.test/u/free\n" || r.stderr != signupNote+"\n" {
		t.Errorf("single free = exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}
	r = runWith(t, p, srv.Client(), "", "mysite", "taken")
	if r.code != 1 || r.stdout != "taken: taken\n" || r.stderr != "" {
		t.Errorf("single taken = exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}
	r = runWith(t, p, srv.Client(), "", "mysite", "forbidden")
	if r.code != 2 || r.stdout != "forbidden: unknown (HTTP 403)\n" || r.stderr != "" {
		t.Errorf("single unknown = exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}
	r = runWith(t, p, srv.Client(), "free\ntaken\nnohead\n", "mysite", "-")
	if r.code != 0 || r.stdout != "free https://p.test/u/free\nnohead https://p.test/u/nohead\n" || r.stderr != "Checked 3: 2 free, 1 taken, 0 unknown.\n"+signupNote+"\n" {
		t.Errorf("scan with free names = exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
	}
	r = runWith(t, p, srv.Client(), "taken\nforbidden\n", "mysite", "-")
	if r.code != 2 || r.stdout != "" || strings.Contains(r.stderr, signupNote) {
		t.Errorf("scan without free names = exit %d, stdout %q, stderr %q; want no note", r.code, r.stdout, r.stderr)
	}
	r = runWith(t, "", srv.Client(), "", srv.URL+"/{}", "free")
	if r.stdout != "free: free "+srv.URL+"/free\n" {
		t.Errorf("template argument = stdout %q; want the template as the profile URL", r.stdout)
	}
	if got := fill("https://p.test/u/{}", "a b"); got != "https://p.test/u/a%20b" {
		t.Errorf("fill escaped %q, want a%%20b", got)
	}
}

// The help screen follows the shared standard: three header lines, then the
// sections the renderer accepts, with the standard rows left to the renderer.
func TestHelpDocFollowsTheStandard(t *testing.T) {
	if err := helpDoc().Check(); err != nil {
		t.Fatal(err)
	}
}
