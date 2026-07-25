package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type fakeFileInfo struct {
	mode os.FileMode
}

func (f fakeFileInfo) Name() string       { return "input" }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() os.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeFileInfo) Sys() any           { return nil }

type failingWriteCloser struct {
	writeErr error
	closeErr error
	closed   bool
}

func (f *failingWriteCloser) Write([]byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return 0, nil
}

func (f *failingWriteCloser) Close() error {
	f.closed = true
	return f.closeErr
}

func restoreGlobals(t *testing.T) {
	t.Helper()
	oldEvaluate := evaluateSymlinks
	oldStat := statFile
	oldRead := readFile
	oldTemp := createTempFile
	oldOutput := createOutputFile
	oldRemove := removeFile
	oldOpen := openInBrowser
	oldRender := renderSource
	t.Cleanup(func() {
		evaluateSymlinks = oldEvaluate
		statFile = oldStat
		readFile = oldRead
		createTempFile = oldTemp
		createOutputFile = oldOutput
		removeFile = oldRemove
		openInBrowser = oldOpen
		renderSource = oldRender
	})
}

func writeInput(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRenderGFMAndOmitRawHTML(t *testing.T) {
	source := []byte(`# Heading

~~gone~~

| A | B |
| - | - |
| 1 | 2 |

- [x] done

https://example.com

[unsafe](javascript:alert("unsafe link"))

<script>alert("unsafe")</script>
`)
	body, err := renderMarkdown(source)
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	for _, want := range []string{"<table>", "<del>gone</del>", `type="checkbox"`, `href="https://example.com"`} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered body missing %q:\n%s", want, got)
		}
	}
	for _, unsafe := range []string{"<script", `alert("unsafe")`, `href="javascript:`} {
		if strings.Contains(got, unsafe) {
			t.Errorf("rendered body passed through %q:\n%s", unsafe, got)
		}
	}
	if !strings.Contains(got, "raw HTML omitted") {
		t.Errorf("rendered body missing inert omission comment:\n%s", got)
	}
}

func TestBuildDocument(t *testing.T) {
	path := writeInput(t, `title<&".md`, "# Hello")
	page, err := buildDocument([]byte("# Hello"), path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(page)
	for _, want := range []string{
		`<body class="markdown-body">`,
		`max-width: 980px`,
		`padding: 45px`,
		`@media (prefers-color-scheme: dark)`,
		`@media (prefers-color-scheme: light)`,
		`background-color: #0d1117`,
		`<title>title&lt;&amp;&#34;.md</title>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("document missing %q", want)
		}
	}
}

func TestFileBaseURL(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "space # percent% ü")
	path := filepath.Join(dir, "input.md")
	base, err := fileBaseURL(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(base)
	for _, want := range []string{"file:", "space%20", "%23", "%25", "%C3%BC", "/"} {
		if !strings.Contains(got, want) {
			t.Errorf("base URL %q missing %q", got, want)
		}
	}
	if !strings.HasSuffix(got, "/") {
		t.Errorf("base URL %q lacks trailing slash", got)
	}

	page, err := buildDocument([]byte(`[relative](docs/a.md) [web](https://example.com) [mail](mailto:a@example.com) [fragment](#part) [root](/root)`), path)
	if err != nil {
		t.Fatal(err)
	}
	html := string(page)
	for _, ref := range []string{`href="docs/a.md"`, `href="https://example.com"`, `href="mailto:a@example.com"`, `href="#part"`, `href="/root"`} {
		if !strings.Contains(html, ref) {
			t.Errorf("document changed or omitted reference %q", ref)
		}
	}
}

func TestParseUsagePrecedenceAndLiteralPath(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"-h"},
		{"-?"},
		{"--help"},
		{"-v"},
		{"--version"},
		{"bad", "--help", "extra"},
	} {
		_, show, err := parseArgs(args)
		if err != nil || !show {
			t.Errorf("parseArgs(%q) = show %v, err %v", args, show, err)
		}
	}
	opts, show, err := parseArgs([]string{"--", "--help"})
	if err != nil || show || opts.input != "--help" {
		t.Fatalf("literal help path = %#v, show %v, err %v", opts, show, err)
	}
}

func TestParseOutputFormsAndFailures(t *testing.T) {
	for _, args := range [][]string{
		{"-o", "out.html", "in.md"},
		{"--output", "out.html", "in.md"},
		{"-o=out.html", "in.md"},
		{"--output=out.html", "in.md"},
	} {
		opts, show, err := parseArgs(args)
		if err != nil || show || opts.input != "in.md" || opts.output != "out.html" {
			t.Errorf("parseArgs(%q) = %#v, show %v, err %v", args, opts, show, err)
		}
	}
	for _, args := range [][]string{
		{"-o"},
		{"-o="},
		{"--output="},
		{"-o", "one.html", "--output", "two.html", "in.md"},
		{"--unknown", "in.md"},
		{"a.md", "b.md"},
	} {
		if _, _, err := parseArgs(args); err == nil || !strings.Contains(err.Error(), "see mdview --help") {
			t.Errorf("parseArgs(%q) error = %v", args, err)
		}
	}
}

func TestRunCLIUsageAndErrorStatus(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := runCLI(nil, &stdout, &stderr); status != 0 {
		t.Fatalf("usage status = %d", status)
	}
	if !strings.Contains(stdout.String(), "mdview v0.1.0") ||
		!strings.Contains(stdout.String(), "mdview [-o FILE] FILE") ||
		!strings.Contains(stdout.String(), "-o, --output FILE") {
		t.Errorf("usage missing contract:\n%s", stdout.String())
	}

	stdout.Reset()
	if status := runCLI([]string{"a", "b"}, &stdout, &stderr); status == 0 {
		t.Fatal("invalid arguments returned success")
	}
	if !strings.Contains(stderr.String(), "mdview: expected FILE (see mdview --help)") {
		t.Errorf("stderr missing prefix and recovery hint: %s", stderr.String())
	}
}

func TestResolveInputTypesAndSymlink(t *testing.T) {
	restoreGlobals(t)
	evaluateSymlinks = func(string) (string, error) { return "resolved.md", nil }
	for name, mode := range map[string]os.FileMode{
		"directory": os.ModeDir,
		"pipe":      os.ModeNamedPipe,
		"socket":    os.ModeSocket,
		"device":    os.ModeDevice,
	} {
		t.Run(name, func(t *testing.T) {
			statFile = func(string) (os.FileInfo, error) { return fakeFileInfo{mode: mode}, nil }
			if _, err := resolveInput("input"); err == nil || !strings.Contains(err.Error(), "not a regular file") {
				t.Errorf("resolveInput error = %v", err)
			}
		})
	}

	target := writeInput(t, "target.anything", "")
	link := filepath.Join(t.TempDir(), "link.md")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	evaluateSymlinks = filepath.EvalSymlinks
	statFile = os.Stat
	resolved, err := resolveInput(link)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(target)
	want, _ = filepath.Abs(want)
	if resolved != want {
		t.Errorf("resolved symlink = %q, want %q", resolved, want)
	}
}

func TestResolveMissingAndDanglingInput(t *testing.T) {
	for _, path := range []string{filepath.Join(t.TempDir(), "missing.md")} {
		if _, err := resolveInput(path); err == nil || !strings.Contains(err.Error(), "provide an existing readable file") {
			t.Errorf("missing input error = %v", err)
		}
	}
	link := filepath.Join(t.TempDir(), "dangling.md")
	if err := os.Symlink(filepath.Join(t.TempDir(), "absent.md"), link); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveInput(link); err == nil {
		t.Fatal("dangling symlink accepted")
	}
}

func TestInjectedReadFailure(t *testing.T) {
	restoreGlobals(t)
	path := writeInput(t, "input.md", "# input")
	readFile = func(string) ([]byte, error) { return nil, errors.New("read failure") }
	err := run([]string{path}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "reading input") || !strings.Contains(err.Error(), "verify the file is readable") {
		t.Fatalf("read error = %v", err)
	}
}

func TestPersistentOutput(t *testing.T) {
	restoreGlobals(t)
	input := writeInput(t, "input.md", "# Hello")
	output := filepath.Join(t.TempDir(), "page.custom")
	opened := 0
	openInBrowser = func(string) error { opened++; return nil }
	var stdout bytes.Buffer
	if err := run([]string{"-o", output, input}, &stdout); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "Created: "+output+"\n" {
		t.Errorf("stdout = %q", stdout.String())
	}
	if opened != 0 {
		t.Errorf("browser opened %d times", opened)
	}
	info, err := os.Stat(output)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o600 != 0o600 {
		t.Errorf("output mode = %o", info.Mode().Perm())
	}
	if err := run([]string{"-o", output, input}, io.Discard); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("overwrite error = %v", err)
	}
}

func TestPersistentOutputRefusesExistingTypesAndMissingParent(t *testing.T) {
	input := writeInput(t, "input.md", "# Hello")
	dir := t.TempDir()
	regular := filepath.Join(dir, "regular")
	if err := os.WriteFile(regular, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "target")
	symlink := filepath.Join(dir, "symlink")
	if err := os.Symlink(target, symlink); err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{regular, dir, symlink} {
		if err := run([]string{"-o", output, input}, io.Discard); err == nil || !strings.Contains(err.Error(), "already exists") {
			t.Errorf("output %q error = %v", output, err)
		}
	}
	missingParent := filepath.Join(dir, "missing", "out.html")
	if err := run([]string{"-o", missingParent, input}, io.Discard); err == nil || !strings.Contains(err.Error(), "parent exists and is writable") {
		t.Errorf("missing parent error = %v", err)
	}
}

func TestPersistentOutputCleansFailuresAndRequestsExclusiveMode(t *testing.T) {
	restoreGlobals(t)
	input := writeInput(t, "input.md", "# Hello")
	output := filepath.Join(t.TempDir(), "out.html")
	renderSource = func([]byte) ([]byte, error) { return nil, errors.New("render failed") }
	if err := run([]string{"-o", output, input}, io.Discard); err == nil {
		t.Fatal("render failure returned success")
	}
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("partial output remains: %v", err)
	}

	renderSource = renderMarkdown
	var capturedPath string
	var capturedFlag int
	var capturedMode os.FileMode
	createOutputFile = func(path string, flag int, mode os.FileMode) (io.WriteCloser, error) {
		capturedPath = path
		capturedFlag = flag
		capturedMode = mode
		return nil, os.ErrExist
	}
	if err := run([]string{"-o", "relative.html", input}, io.Discard); err == nil {
		t.Fatal("create failure returned success")
	}
	if capturedPath != "relative.html" {
		t.Errorf("create path = %q", capturedPath)
	}
	if capturedFlag != os.O_WRONLY|os.O_CREATE|os.O_EXCL {
		t.Errorf("create flag = %d", capturedFlag)
	}
	if capturedMode != 0o644 {
		t.Errorf("create mode = %o", capturedMode)
	}
}

func TestWriteAndCloseFailuresRemoveOutput(t *testing.T) {
	for name, writer := range map[string]*failingWriteCloser{
		"write": {writeErr: errors.New("write failed")},
		"close": {closeErr: errors.New("close failed")},
	} {
		t.Run(name, func(t *testing.T) {
			restoreGlobals(t)
			input := writeInput(t, "input.md", "# Hello")
			output := filepath.Join(t.TempDir(), "out.html")
			if err := os.WriteFile(output, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			createOutputFile = func(string, int, os.FileMode) (io.WriteCloser, error) { return writer, nil }
			removeFile = func(path string) error { return os.Remove(output) }
			if err := writePersistent(output+".logical", []byte("# Hello"), input, io.Discard); err == nil {
				t.Fatal("failure returned success")
			}
			if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("partial output remains: %v", err)
			}
		})
	}
}

func TestCleanupFailureHasRecoveryGuidance(t *testing.T) {
	restoreGlobals(t)
	input := writeInput(t, "input.md", "# Hello")
	output := filepath.Join(t.TempDir(), "out.html")
	renderSource = func([]byte) ([]byte, error) { return nil, errors.New("render failed") }
	removeFile = func(string) error { return errors.New("remove failed") }
	err := run([]string{"-o", output, input}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "removing partial output") ||
		!strings.Contains(err.Error(), "remove it manually") {
		t.Fatalf("cleanup error = %v", err)
	}
}

func TestTemporaryOutputLifecycleAndOpener(t *testing.T) {
	for _, openerErr := range []error{nil, errors.New("open failed")} {
		t.Run(fmt.Sprint(openerErr), func(t *testing.T) {
			restoreGlobals(t)
			input := writeInput(t, "input.md", "# Hello")
			var opened string
			openInBrowser = func(path string) error {
				opened = path
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
					t.Errorf("temporary mode = %o", info.Mode().Perm())
				}
				return openerErr
			}
			err := run([]string{input}, io.Discard)
			if (err != nil) != (openerErr != nil) {
				t.Fatalf("run error = %v, opener error = %v", err, openerErr)
			}
			if !filepath.IsAbs(opened) || !strings.HasPrefix(filepath.Base(opened), "mdview-") ||
				filepath.Ext(opened) != ".html" {
				t.Errorf("opened path = %q", opened)
			}
			_, statErr := os.Stat(opened)
			if openerErr == nil {
				if statErr != nil {
					t.Errorf("successful temporary output removed: %v", statErr)
				}
				t.Cleanup(func() { _ = os.Remove(opened) })
			} else if !errors.Is(statErr, os.ErrNotExist) {
				t.Errorf("failed temporary output remains: %v", statErr)
			}
		})
	}
}

func TestProgramIdentityAndStylesheetChecksum(t *testing.T) {
	if programName != "mdview" || programVersion != "0.1.0" {
		t.Errorf("identity = %s %s", programName, programVersion)
	}
	got := fmt.Sprintf("%x", sha256.Sum256([]byte(stylesheet)))
	if got != stylesheetSHA256 {
		t.Errorf("stylesheet SHA-256 = %s, want %s", got, stylesheetSHA256)
	}
}
