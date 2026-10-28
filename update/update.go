// Package update tells a program, at most once per interval, whether a newer
// release of its Go module exists on the module proxy. It only reports: it
// prints nothing, runs no subprocess, and installs nothing.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	defaultProxy    = "https://proxy.golang.org"
	defaultInterval = 24 * time.Hour
	defaultTimeout  = 2 * time.Second

	// retryAfter is how long a failed check waits before the next request.
	retryAfter = time.Hour

	// maxBody caps the proxy response read; an @latest reply is far smaller.
	maxBody = 64 << 10

	// SkippedRecentFailure is the Skipped reason for a call within retryAfter
	// of a failed check.
	SkippedRecentFailure = "recent check failed"
)

// Options configures one Check.
type Options struct {
	Module    string        // module path, such as github.com/queone/macfit
	Current   string        // running version, vMAJOR.MINOR.PATCH
	CacheFile string        // path of the cache file
	Interval  time.Duration // time between proxy requests; 24 hours when zero
	Timeout   time.Duration // bound on one proxy request; 2 seconds when zero

	Client *http.Client     // HTTP client; a plain client when nil
	Proxy  string           // proxy base URL; replaces all GOPROXY handling when set
	Now    func() time.Time // clock; time.Now when nil
}

// Result reports the outcome of one Check.
type Result struct {
	Current   string
	Latest    string // newest release, strict vMAJOR.MINOR.PATCH, or ""
	Available bool   // Latest is newer than Current
	CheckedAt time.Time
	FromCache bool
	Skipped   string // reason no check ran, or ""
}

type cache struct {
	Module    string    `json:"module"`
	Latest    string    `json:"latest"`
	CheckedAt time.Time `json:"checked_at"`
	FailedAt  time.Time `json:"failed_at"`
}

var strict = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// parse returns the three numbers of a strict vMAJOR.MINOR.PATCH version.
func parse(v string) ([3]uint64, bool) {
	var n [3]uint64
	m := strict.FindStringSubmatch(v)
	if m == nil {
		return n, false
	}
	for i := range n {
		x, err := strconv.ParseUint(m[i+1], 10, 64)
		if err != nil {
			return n, false
		}
		n[i] = x
	}
	return n, true
}

// Newer reports whether version a is newer than version b. Both must be strict
// vMAJOR.MINOR.PATCH; any other form is never newer.
func Newer(a, b string) bool {
	x, ok := parse(a)
	if !ok {
		return false
	}
	y, ok := parse(b)
	if !ok {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return x[i] > y[i]
		}
	}
	return false
}

// CacheDir returns the per-program cache directory, os.UserCacheDir()/program.
func CacheDir(program string) (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, program), nil
}

// escapePath applies the module proxy case encoding: each uppercase letter
// becomes '!' followed by its lowercase form.
func escapePath(module string) string {
	var b strings.Builder
	for _, r := range module {
		if r >= 'A' && r <= 'Z' {
			b.WriteByte('!')
			r += 'a' - 'A'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// proxyURL picks the proxy base URL, or returns a skip reason.
func proxyURL(o Options) (base, skip string) {
	if o.Proxy != "" {
		return strings.TrimRight(o.Proxy, "/"), ""
	}
	env := os.Getenv("GOPROXY")
	if env == "" {
		return defaultProxy, ""
	}
	first := env
	if i := strings.IndexAny(env, ",|"); i >= 0 {
		first = env[:i]
	}
	first = strings.TrimSpace(first)
	u, err := url.Parse(first)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Sprintf("GOPROXY entry %q is not an http or https proxy", first)
	}
	return strings.TrimRight(first, "/"), ""
}

// readCache returns the cache for module, or an empty cache when the file is
// missing, corrupt, for another module, or dated after now.
func readCache(path, module string, now time.Time) cache {
	data, err := os.ReadFile(path)
	if err != nil {
		return cache{}
	}
	var c cache
	if json.Unmarshal(data, &c) != nil || c.Module != module || c.CheckedAt.After(now) {
		return cache{}
	}
	if _, ok := parse(c.Latest); !ok {
		c.Latest = ""
	}
	if c.FailedAt.After(now) {
		c.FailedAt = time.Time{}
	}
	return c
}

// writeCache writes c to path atomically with mode 0600, creating a missing
// directory with mode 0700.
func writeCache(path string, c cache) error {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".update-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// fetch asks the proxy for the module's @latest version.
func fetch(o Options, base string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), o.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/"+escapePath(o.Module)+"/@latest", nil)
	if err != nil {
		return "", err
	}
	resp, err := o.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("proxy answered %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return "", err
	}
	if len(body) > maxBody {
		return "", fmt.Errorf("proxy reply exceeds %d bytes", maxBody)
	}
	var info struct{ Version string }
	if err := json.Unmarshal(body, &info); err != nil {
		return "", fmt.Errorf("decode proxy reply: %w", err)
	}
	return info.Version, nil
}

// Check reports whether a newer release of o.Module than o.Current exists.
// Within o.Interval of the last successful check it answers from the cache
// with no network request. Within one hour of a failed check it skips the
// request and returns the last good result from the cache.
func Check(o Options) (Result, error) {
	r := Result{Current: o.Current}
	if os.Getenv("NO_UPDATE_CHECK") != "" {
		r.Skipped = "NO_UPDATE_CHECK is set"
		return r, nil
	}
	if _, ok := parse(o.Current); !ok {
		r.Skipped = fmt.Sprintf("version %q is not a release", o.Current)
		return r, nil
	}
	base, skip := proxyURL(o)
	if skip != "" {
		r.Skipped = skip
		return r, nil
	}
	if o.Module == "" || o.CacheFile == "" {
		return r, errors.New("check for update: module and cache file are required")
	}
	if o.Interval <= 0 {
		o.Interval = defaultInterval
	}
	if o.Timeout <= 0 {
		o.Timeout = defaultTimeout
	}
	if o.Client == nil {
		o.Client = &http.Client{}
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	now := o.Now()

	c := readCache(o.CacheFile, o.Module, now)
	cached := Result{
		Current:   o.Current,
		Latest:    c.Latest,
		Available: Newer(c.Latest, o.Current),
		CheckedAt: c.CheckedAt,
		FromCache: true,
	}
	if !c.CheckedAt.IsZero() && now.Sub(c.CheckedAt) < o.Interval {
		return cached, nil
	}
	if !c.FailedAt.IsZero() && now.Sub(c.FailedAt) < retryAfter {
		cached.Skipped = SkippedRecentFailure
		return cached, nil
	}

	v, err := fetch(o, base)
	if err != nil {
		c.Module = o.Module
		c.FailedAt = now
		err = fmt.Errorf("check for update of %s: %w", o.Module, err)
		if werr := writeCache(o.CacheFile, c); werr != nil {
			err = errors.Join(err, fmt.Errorf("write update cache: %w", werr))
		}
		return r, err
	}
	if _, ok := parse(v); !ok {
		v = ""
	}
	r.Latest = v
	r.Available = Newer(v, o.Current)
	r.CheckedAt = now
	if err := writeCache(o.CacheFile, cache{Module: o.Module, Latest: v, CheckedAt: now}); err != nil {
		return r, fmt.Errorf("write update cache: %w", err)
	}
	return r, nil
}
