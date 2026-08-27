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

	"github.com/PuerkitoBio/goquery"
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

func parseRenderedFragment(t *testing.T, rendered []byte) *goquery.Selection {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<div id=\"root\">" + string(rendered) + "</div>"))
	if err != nil {
		t.Fatal(err)
	}
	return doc.Find("#root")
}

func TestRenderDetailsWithNestedGFM(t *testing.T) {
	source := []byte(`<details>
<summary>Raw samples</summary>

| A | B |
| --- | --- |
| 1 | 2 |

</details>`)
	body, err := renderMarkdown(source)
	if err != nil {
		t.Fatal(err)
	}
	root := parseRenderedFragment(t, body)
	details := root.ChildrenFiltered("details")
	if details.Length() != 1 {
		t.Fatalf("details count = %d, want 1: %s", details.Length(), body)
	}
	detail := details.First()
	if _, ok := detail.Attr("open"); ok {
		t.Fatalf("details unexpectedly has open attribute: %s", body)
	}
	if got := detail.ChildrenFiltered("summary").First().Text(); got != "Raw samples" {
		t.Fatalf("summary = %q, want %q", got, "Raw samples")
	}
	if detail.Find("table").Length() != 1 {
		t.Fatalf("nested table missing from details: %s", body)
	}
}

func TestRenderDetailsStripsAttributesAndUnsafeHTML(t *testing.T) {
	source := []byte(`<script>alert("unsafe")</script>
<div>raw</div>
<iframe src="https://example.com"></iframe>
<style>.x{}</style>
<details open onclick="alert('unsafe')"><summary style="color:red">Label</summary>
content
</details>
[unsafe](javascript:alert("unsafe link"))`)
	body, err := renderMarkdown(source)
	if err != nil {
		t.Fatal(err)
	}
	root := parseRenderedFragment(t, body)
	for _, selector := range []string{"script", "div", "iframe", "style", "[onclick]", "[style]", "[href^=javascript]"} {
		if root.Find(selector).Length() != 0 {
			t.Fatalf("unsafe selector %q survived: %s", selector, body)
		}
	}
	details := root.Find("details")
	if details.Length() != 1 {
		t.Fatalf("details count = %d, want 1: %s", details.Length(), body)
	}
	details.Each(func(_ int, selection *goquery.Selection) {
		if len(selection.Nodes[0].Attr) != 0 {
			t.Errorf("details attributes survived: %s", body)
		}
	})
}

func TestRenderDetailsRemainIndependent(t *testing.T) {
	source := []byte(`<details><summary>Raw samples</summary>

| A |
| --- |
| 1 |

</details>

<details><summary>Artifact identity</summary>

| B |
| --- |
| 2 |

</details>`)
	body, err := renderMarkdown(source)
	if err != nil {
		t.Fatal(err)
	}
	root := parseRenderedFragment(t, body)
	details := root.ChildrenFiltered("details")
	if details.Length() != 2 {
		t.Fatalf("sibling details count = %d, want 2: %s", details.Length(), body)
	}
	wantLabels := []string{"Raw samples", "Artifact identity"}
	details.Each(func(i int, selection *goquery.Selection) {
		if _, ok := selection.Attr("open"); ok {
			t.Errorf("details %d unexpectedly open: %s", i, body)
		}
		summary := selection.ChildrenFiltered("summary")
		if summary.Length() != 1 || summary.Text() != wantLabels[i] {
			t.Errorf("details %d summary = %q, want %q", i, summary.Text(), wantLabels[i])
		}
		if selection.Find("table").Length() != 1 {
			t.Errorf("details %d table missing: %s", i, body)
		}
	})
}

func TestRenderDetailsEdgeCases(t *testing.T) {
	mixed := []byte(`<DETAILS data-x="discard"><SUMMARY>Repeated</SUMMARY>
body
<summary>Repeated</summary>
<details class="nested"><summary>Nested</summary>nested body</details>
</DETAILS>

<summary>Orphan</summary>`)
	body, err := renderMarkdown(mixed)
	if err != nil {
		t.Fatal(err)
	}
	root := parseRenderedFragment(t, body)
	details := root.Find("details")
	if details.Length() != 2 {
		t.Fatalf("mixed-case nested details count = %d, want 2: %s", details.Length(), body)
	}
	if details.First().ChildrenFiltered("summary").Text() != "Repeated" ||
		details.Last().ChildrenFiltered("summary").Text() != "Nested" {
		t.Fatalf("summary labels changed: %s", body)
	}
	if root.Find("details[open], details[*]").Length() != 0 {
		t.Fatalf("details attributes survived: %s", body)
	}

	unclosed, err := renderMarkdown([]byte(`<details><summary>Lost</summary>body`))
	if err != nil {
		t.Fatal(err)
	}
	if root := parseRenderedFragment(t, unclosed); root.Find("details,summary").Length() != 0 {
		t.Fatalf("unclosed disclosure emitted HTML: %s", unclosed)
	}
}

func TestRenderDetailsDoesNotRewriteCode(t *testing.T) {
	source := []byte("```html\n<details open><summary>literal</summary></details>\n```\n\n" +
		"    <details><summary>indented</summary></details>\n\n" +
		"Inline `<details open>` remains literal.")
	body, err := renderMarkdown(source)
	if err != nil {
		t.Fatal(err)
	}
	root := parseRenderedFragment(t, body)
	if root.Find("details").Length() != 0 {
		t.Fatalf("code sample became a disclosure: %s", body)
	}
	if !strings.Contains(string(body), "&lt;details open&gt;") ||
		!strings.Contains(string(body), "&lt;details&gt;") ||
		!strings.Contains(string(body), "<code>&lt;details open&gt;</code>") {
		t.Fatalf("literal disclosure markup was changed: %s", body)
	}
}

func TestRenderDetailsDoesNotRewriteRawContainersOrComments(t *testing.T) {
	source := []byte(`<!-- <details><summary>comment</summary></details> -->
<script><details><summary>script</summary></details></script>
<style><details><summary>style</summary></details></style>`)
	body, err := renderMarkdown(source)
	if err != nil {
		t.Fatal(err)
	}
	if root := parseRenderedFragment(t, body); root.Find("details, script, style").Length() != 0 {
		t.Fatalf("raw container content became active HTML: %s", body)
	}
}

func TestRenderDetailsRejectsMalformedClosingTag(t *testing.T) {
	body, err := renderMarkdown([]byte(`<details><summary>Label</summary>body</details data-x="unsafe">`))
	if err != nil {
		t.Fatal(err)
	}
	if root := parseRenderedFragment(t, body); root.Find("details, summary").Length() != 0 {
		t.Fatalf("malformed disclosure emitted HTML: %s", body)
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
		!strings.Contains(stdout.String(), "-o, --output FILE") ||
		!strings.Contains(stdout.String(), "print mdview v0.1.0 and exit") {
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

func TestRunCLIVersionAliases(t *testing.T) {
	for _, flag := range []string{"-v", "--version"} {
		t.Run(flag, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if status := runCLI([]string{flag}, &stdout, &stderr); status != 0 {
				t.Fatalf("status = %d, want 0", status)
			}
			if got, want := stdout.String(), "mdview v0.1.0\n"; got != want {
				t.Errorf("stdout = %q, want %q", got, want)
			}
			if got := stderr.String(); got != "" {
				t.Errorf("stderr = %q, want empty", got)
			}
		})
	}
}

func TestRunCLISpecialFlagPrecedence(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if status := runCLI([]string{"--help", "--version"}, &stdout, &stderr); status != 0 {
		t.Fatalf("help-first status = %d, want 0", status)
	}
	if !strings.Contains(stdout.String(), "View GitHub Flavored Markdown") {
		t.Errorf("help-first stdout is not usage: %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if status := runCLI([]string{"--version", "--help"}, &stdout, &stderr); status != 0 {
		t.Fatalf("version-first status = %d, want 0", status)
	}
	if got, want := stdout.String(), "mdview v0.1.0\n"; got != want {
		t.Errorf("version-first stdout = %q, want %q", got, want)
	}
	if got := stderr.String(); got != "" {
		t.Errorf("version-first stderr = %q, want empty", got)
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
