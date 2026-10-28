package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeProxy is an httptest module proxy that counts requests and serves a
// configurable @latest reply.
type fakeProxy struct {
	*httptest.Server
	mu    sync.Mutex
	paths []string
	body  string
	slow  bool
}

func newFakeProxy(t *testing.T, version string) *fakeProxy {
	t.Helper()
	reply, err := json.Marshal(map[string]string{"Version": version, "Time": "2026-10-01T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	p := &fakeProxy{body: string(reply)}
	p.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		p.paths = append(p.paths, r.URL.Path)
		body, slow := p.body, p.slow
		p.mu.Unlock()
		if slow {
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
			return
		}
		fmt.Fprint(w, body)
	}))
	t.Cleanup(p.Close)
	return p
}

func (p *fakeProxy) set(body string, slow bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.body, p.slow = body, slow
}

func (p *fakeProxy) requests() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.paths...)
}

// clock is a settable test clock.
type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

// failTransport fails and counts every request, so a test that expects no
// request never reaches the network.
type failTransport struct{ n int }

func (f *failTransport) RoundTrip(*http.Request) (*http.Response, error) {
	f.n++
	return nil, errors.New("unexpected request")
}

// setup clears the environment that steers Check and points GOPROXY at p.
func setup(t *testing.T, p *fakeProxy) (Options, *clock) {
	t.Helper()
	t.Setenv("NO_UPDATE_CHECK", "")
	t.Setenv("GOPROXY", p.URL)
	clk := &clock{t: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	return Options{
		Module:    "github.com/queone/macfit",
		Current:   "v1.2.0",
		CacheFile: filepath.Join(t.TempDir(), "update.json"),
		Client:    p.Client(),
		Now:       clk.now,
	}, clk
}

func TestCheckWithinIntervalAnswersFromCache(t *testing.T) {
	p := newFakeProxy(t, "v1.3.0")
	o, clk := setup(t, p)
	if _, err := Check(o); err != nil {
		t.Fatal(err)
	}
	clk.t = clk.t.Add(23 * time.Hour)
	r, err := Check(o)
	if err != nil {
		t.Fatal(err)
	}
	if !r.FromCache || r.Latest != "v1.3.0" || !r.Available {
		t.Fatalf("result %+v, want cached v1.3.0 available", r)
	}
	if n := len(p.requests()); n != 1 {
		t.Fatalf("proxy saw %d requests, want 1", n)
	}
}

func TestCheckAfterIntervalAsksProxyOnceAndRewritesCache(t *testing.T) {
	p := newFakeProxy(t, "v1.2.0")
	o, clk := setup(t, p)
	if _, err := Check(o); err != nil {
		t.Fatal(err)
	}
	p.set(`{"Version":"v1.3.0"}`, false)
	clk.t = clk.t.Add(25 * time.Hour)
	r, err := Check(o)
	if err != nil {
		t.Fatal(err)
	}
	if r.FromCache || r.Latest != "v1.3.0" || !r.Available || !r.CheckedAt.Equal(clk.t) {
		t.Fatalf("result %+v, want fresh v1.3.0 available", r)
	}
	paths := p.requests()
	if len(paths) != 2 || paths[1] != "/github.com/queone/macfit/@latest" {
		t.Fatalf("proxy requests %v", paths)
	}
	var c cache
	data, err := os.ReadFile(o.CacheFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatal(err)
	}
	if c.Latest != "v1.3.0" || !c.CheckedAt.Equal(clk.t) {
		t.Fatalf("cache %+v, want v1.3.0 checked at %v", c, clk.t)
	}
}

func TestCheckEscapesUppercaseModulePath(t *testing.T) {
	p := newFakeProxy(t, "v1.3.0")
	o, _ := setup(t, p)
	o.Module = "github.com/Queone/x"
	if _, err := Check(o); err != nil {
		t.Fatal(err)
	}
	if paths := p.requests(); len(paths) != 1 || paths[0] != "/github.com/!queone/x/@latest" {
		t.Fatalf("proxy requests %v", paths)
	}
}

func TestCheckReportsNoUpdateForSameOlderOrUnusableVersions(t *testing.T) {
	for _, tc := range []struct{ proxy, latest string }{
		{"v1.2.0", "v1.2.0"},
		{"v1.1.9", "v1.1.9"},
		{"v1.3.0-rc.1", ""},
		{"v1.3.0\x1b[31m", ""},
		{"1.3.0", ""},
	} {
		t.Run(tc.proxy, func(t *testing.T) {
			p := newFakeProxy(t, tc.proxy)
			o, _ := setup(t, p)
			r, err := Check(o)
			if err != nil {
				t.Fatal(err)
			}
			if r.Available || r.Latest != tc.latest {
				t.Fatalf("result %+v, want Latest %q and no update", r, tc.latest)
			}
		})
	}
}

func TestCheckSkipsWithoutRequest(t *testing.T) {
	for _, tc := range []struct {
		name, noCheck, goproxy, current string
	}{
		{"no update check", "1", "", "v1.2.0"},
		{"goproxy off", "", "off", "v1.2.0"},
		{"goproxy direct", "", "direct,https://proxy.golang.org", "v1.2.0"},
		{"goproxy file", "", "file:///tmp/p", "v1.2.0"},
		{"devel build", "", "", "(devel)"},
		{"pseudo-version", "", "", "v0.0.0-20260101000000-abcdef123456"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("NO_UPDATE_CHECK", tc.noCheck)
			t.Setenv("GOPROXY", tc.goproxy)
			ft := &failTransport{}
			r, err := Check(Options{
				Module:    "github.com/queone/macfit",
				Current:   tc.current,
				CacheFile: filepath.Join(t.TempDir(), "update.json"),
				Client:    &http.Client{Transport: ft},
			})
			if err != nil {
				t.Fatal(err)
			}
			if r.Skipped == "" || ft.n != 0 {
				t.Fatalf("result %+v after %d requests, want a skip and none", r, ft.n)
			}
		})
	}
}

func TestCheckTimesOutThenWaitsAnHourKeepingLastGoodVersion(t *testing.T) {
	p := newFakeProxy(t, "v1.3.0")
	o, clk := setup(t, p)
	o.Client = &http.Client{}
	o.Timeout = 100 * time.Millisecond
	if _, err := Check(o); err != nil {
		t.Fatal(err)
	}
	p.set("", true)
	clk.t = clk.t.Add(25 * time.Hour)
	start := time.Now()
	if _, err := Check(o); err == nil {
		t.Fatal("slow proxy: want an error")
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("slow proxy took %v, want about the 100ms timeout", d)
	}
	clk.t = clk.t.Add(30 * time.Minute)
	r, err := Check(o)
	if err != nil {
		t.Fatal(err)
	}
	if r.Skipped != SkippedRecentFailure || !r.FromCache || r.Latest != "v1.3.0" || !r.Available {
		t.Fatalf("result %+v, want recent-failure skip with cached v1.3.0", r)
	}
	if n := len(p.requests()); n != 2 {
		t.Fatalf("proxy saw %d requests, want 2", n)
	}
	clk.t = clk.t.Add(31 * time.Minute)
	p.set(`{"Version":"v1.4.0"}`, false)
	if r, err := Check(o); err != nil || r.Latest != "v1.4.0" {
		t.Fatalf("after the hour: result %+v err %v, want fresh v1.4.0", r, err)
	}
}

func TestNewer(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"v2.0.0", "v1.9.9", true},
		{"v1.10.0", "v1.9.0", true},
		{"v1.2.10", "v1.2.9", true},
		{"v1.9.9", "v2.0.0", false},
		{"v1.2.3", "v1.2.3", false},
		{"1.3.0", "v1.2.0", false},
		{"v1.3.0", "1.2.0", false},
		{"v1.3.0-rc.1", "v1.2.0", false},
		{"v1.3.0", "v1.2.0-rc.1", false},
		{"v1.3", "v1.2.0", false},
		{"v01.3.0", "v1.2.0", false},
		{"", "v1.2.0", false},
		{"(devel)", "v1.2.0", false},
		{"v1.3.0+meta", "v1.2.0", false},
	} {
		if got := Newer(tc.a, tc.b); got != tc.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCacheModesAndCacheDir(t *testing.T) {
	p := newFakeProxy(t, "v1.3.0")
	o, _ := setup(t, p)
	dir := filepath.Join(t.TempDir(), "new", "macfit")
	o.CacheFile = filepath.Join(dir, "update.json")
	if _, err := Check(o); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]os.FileMode{o.CacheFile: 0o600, dir: 0o700} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != want {
			t.Fatalf("%s mode %o, want %o", path, info.Mode().Perm(), want)
		}
	}

	existing := filepath.Join(t.TempDir(), "existing")
	if err := os.Mkdir(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	o.CacheFile = filepath.Join(existing, "update.json")
	if _, err := Check(o); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(existing); err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("existing directory mode %v %v, want 755", info.Mode().Perm(), err)
	}

	base, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := CacheDir("macfit"); err != nil || got != filepath.Join(base, "macfit") {
		t.Fatalf("CacheDir = %q %v, want %q", got, err, filepath.Join(base, "macfit"))
	}
}

func TestOptionsProxyReplacesGOPROXY(t *testing.T) {
	p := newFakeProxy(t, "v1.3.0")
	o, _ := setup(t, p)
	t.Setenv("GOPROXY", "off")
	o.Proxy = p.URL
	r, err := Check(o)
	if err != nil {
		t.Fatal(err)
	}
	if r.Skipped != "" || len(p.requests()) != 1 {
		t.Fatalf("result %+v after %d requests, want one request", r, len(p.requests()))
	}
}

func TestCheckTreatsUnusableCacheAsStale(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct{ name, content string }{
		{"corrupt", "not json"},
		{"other module", fmt.Sprintf(`{"module":"github.com/queone/other","latest":"v1.3.0","checked_at":%q}`,
			now.Add(-time.Hour).Format(time.RFC3339))},
		{"future", fmt.Sprintf(`{"module":"github.com/queone/macfit","latest":"v1.3.0","checked_at":%q}`,
			now.Add(time.Hour).Format(time.RFC3339))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newFakeProxy(t, "v1.3.0")
			o, _ := setup(t, p)
			if err := os.WriteFile(o.CacheFile, []byte(tc.content), 0o600); err != nil {
				t.Fatal(err)
			}
			r, err := Check(o)
			if err != nil {
				t.Fatal(err)
			}
			if r.FromCache || len(p.requests()) != 1 {
				t.Fatalf("result %+v after %d requests, want one fresh request", r, len(p.requests()))
			}
		})
	}
}

func TestCheckRejectsOversizedReply(t *testing.T) {
	p := newFakeProxy(t, "v1.3.0")
	o, _ := setup(t, p)
	p.set(`{"Version":"v1.3.0","Pad":"`+strings.Repeat("x", maxBody)+`"}`, false)
	if _, err := Check(o); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized reply: err %v, want a size error", err)
	}
}
