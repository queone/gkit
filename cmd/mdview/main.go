package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/skratchdot/open-golang/open"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

const (
	programName    = "mdview"
	programVersion = "0.1.0"

	// Source: https://raw.githubusercontent.com/sindresorhus/github-markdown-css/v5.9.0/github-markdown.css
	// SHA-256: 6112686f954db5d3806fb96116d2ab20ad3018469ab1015c587fd8efe7d25cf4
	stylesheetSHA256 = "6112686f954db5d3806fb96116d2ab20ad3018469ab1015c587fd8efe7d25cf4"
)

//go:embed github-markdown.css
var stylesheet string

var (
	evaluateSymlinks = filepath.EvalSymlinks
	statFile         = os.Stat
	readFile         = os.ReadFile
	createTempFile   = os.CreateTemp
	createOutputFile = func(path string, flag int, perm os.FileMode) (io.WriteCloser, error) {
		return os.OpenFile(path, flag, perm)
	}
	removeFile    = os.Remove
	openInBrowser = open.Run
	renderSource  = renderMarkdown
)

type options struct {
	input  string
	output string
}

type documentData struct {
	Title      string
	BaseURL    template.URL
	Stylesheet template.CSS
	Body       template.HTML
}

const pageTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<base href="{{.BaseURL}}">
<title>{{.Title}}</title>
<style>
{{.Stylesheet}}
html {
  background-color: #ffffff;
}
@media (prefers-color-scheme: dark) {
  html {
    background-color: #0d1117;
  }
}
.markdown-body {
  box-sizing: border-box;
  min-width: 200px;
  max-width: 980px;
  margin: 0 auto;
  padding: 45px;
}
@media (max-width: 767px) {
  .markdown-body {
    padding: 15px;
  }
}
</style>
</head>
<body class="markdown-body">
{{.Body}}
</body>
</html>
`

func usage() string {
	return fmt.Sprintf(`%s v%s
View GitHub Flavored Markdown in a browser or write it as HTML.

Usage
  %s [-o FILE] FILE

Options
  -o, --output FILE  write HTML to FILE without opening a browser
  -v, --version      show this help message and exit
  -h, -?, --help     show this help message and exit
`, programName, programVersion, programName)
}

func isHelp(arg string) bool {
	return arg == "-h" || arg == "-?" || arg == "--help" ||
		arg == "-v" || arg == "--version"
}

func parseArgs(args []string) (options, bool, error) {
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if isHelp(arg) {
			return options{}, true, nil
		}
	}
	if len(args) == 0 {
		return options{}, true, nil
	}

	var opts options
	var positional []string
	literal := false
	outputSet := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if literal {
			positional = append(positional, arg)
			continue
		}
		switch {
		case arg == "--":
			literal = true
		case arg == "-o" || arg == "--output":
			if outputSet {
				return options{}, false, fmt.Errorf("output option specified more than once (see %s --help)", programName)
			}
			if i+1 >= len(args) || args[i+1] == "" {
				return options{}, false, fmt.Errorf("%s requires a non-empty FILE (see %s --help)", arg, programName)
			}
			i++
			opts.output = args[i]
			outputSet = true
		case strings.HasPrefix(arg, "-o="):
			if outputSet {
				return options{}, false, fmt.Errorf("output option specified more than once (see %s --help)", programName)
			}
			opts.output = strings.TrimPrefix(arg, "-o=")
			if opts.output == "" {
				return options{}, false, fmt.Errorf("-o requires a non-empty FILE (see %s --help)", programName)
			}
			outputSet = true
		case strings.HasPrefix(arg, "--output="):
			if outputSet {
				return options{}, false, fmt.Errorf("output option specified more than once (see %s --help)", programName)
			}
			opts.output = strings.TrimPrefix(arg, "--output=")
			if opts.output == "" {
				return options{}, false, fmt.Errorf("--output requires a non-empty FILE (see %s --help)", programName)
			}
			outputSet = true
		case strings.HasPrefix(arg, "-"):
			return options{}, false, fmt.Errorf("unknown flag %q (see %s --help)", arg, programName)
		default:
			positional = append(positional, arg)
		}
	}
	if len(positional) != 1 {
		return options{}, false, fmt.Errorf("expected FILE (see %s --help)", programName)
	}
	opts.input = positional[0]
	return opts, false, nil
}

func resolveInput(path string) (string, error) {
	resolved, err := evaluateSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolving input %q: %w; provide an existing readable file", path, err)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("resolving absolute input path %q: %w; verify the path", path, err)
	}
	info, err := statFile(resolved)
	if err != nil {
		return "", fmt.Errorf("checking input %q: %w; verify the file is readable", path, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("checking input %q: not a regular file; provide a regular file", path)
	}
	return resolved, nil
}

func fileBaseURL(path string) (template.URL, error) {
	dir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return "", fmt.Errorf("resolving resource directory for %q: %w; verify the path", path, err)
	}
	slash := filepath.ToSlash(dir)
	if volume := filepath.VolumeName(dir); volume != "" && !strings.HasPrefix(slash, "/") {
		slash = "/" + slash
	}
	if !strings.HasSuffix(slash, "/") {
		slash += "/"
	}
	return template.URL((&url.URL{Scheme: "file", Path: slash}).String()), nil
}

func renderMarkdown(source []byte) ([]byte, error) {
	var body bytes.Buffer
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	if err := md.Convert(source, &body); err != nil {
		return nil, fmt.Errorf("rendering Markdown: %w; verify the input content", err)
	}
	return body.Bytes(), nil
}

func buildDocument(source []byte, resolvedInput string) ([]byte, error) {
	body, err := renderSource(source)
	if err != nil {
		return nil, err
	}
	baseURL, err := fileBaseURL(resolvedInput)
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New("page").Parse(pageTemplate)
	if err != nil {
		return nil, fmt.Errorf("preparing HTML template: %w; reinstall %s", err, programName)
	}
	var page bytes.Buffer
	if err := tmpl.Execute(&page, documentData{
		Title:      filepath.Base(resolvedInput),
		BaseURL:    baseURL,
		Stylesheet: template.CSS(stylesheet),
		Body:       template.HTML(body),
	}); err != nil {
		return nil, fmt.Errorf("building HTML document: %w; verify the input content", err)
	}
	return page.Bytes(), nil
}

func writeDocument(dst io.WriteCloser, path string, source []byte, resolvedInput string) error {
	page, err := buildDocument(source, resolvedInput)
	if err != nil {
		_ = dst.Close()
		return err
	}
	if _, err := dst.Write(page); err != nil {
		_ = dst.Close()
		return fmt.Errorf("writing HTML output %q: %w; verify available space and permissions", path, err)
	}
	if err := dst.Close(); err != nil {
		return fmt.Errorf("closing HTML output %q: %w; verify the filesystem", path, err)
	}
	return nil
}

func outputExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func removePartial(path string, cause error) error {
	if err := removeFile(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%v; removing partial output %q: %w; remove it manually", cause, path, err)
	}
	return cause
}

func writePersistent(path string, source []byte, resolvedInput string, stdout io.Writer) error {
	exists, err := outputExists(path)
	if err != nil {
		return fmt.Errorf("checking output %q: %w; verify the destination path", path, err)
	}
	if exists {
		return fmt.Errorf("output %q already exists; choose a new path", path)
	}
	dst, err := createOutputFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("creating output %q: %w; verify the parent exists and is writable", path, err)
	}
	if err := writeDocument(dst, path, source, resolvedInput); err != nil {
		return removePartial(path, err)
	}
	fmt.Fprintf(stdout, "Created: %s\n", path)
	return nil
}

func openTemporary(source []byte, resolvedInput string) error {
	dst, err := createTempFile("", "mdview-*.html")
	if err != nil {
		return fmt.Errorf("creating temporary HTML output: %w; verify the temporary directory is writable", err)
	}
	path, err := filepath.Abs(dst.Name())
	if err != nil {
		_ = dst.Close()
		_ = removeFile(dst.Name())
		return fmt.Errorf("resolving temporary HTML path: %w; verify the temporary directory", err)
	}
	if err := writeDocument(dst, path, source, resolvedInput); err != nil {
		return removePartial(path, err)
	}
	if err := openInBrowser(path); err != nil {
		return removePartial(path, fmt.Errorf("opening temporary HTML %q: %w; verify a default browser is configured", path, err))
	}
	return nil
}

func run(args []string, stdout io.Writer) error {
	opts, showUsage, err := parseArgs(args)
	if err != nil {
		return err
	}
	if showUsage {
		fmt.Fprint(stdout, usage())
		return nil
	}
	resolved, err := resolveInput(opts.input)
	if err != nil {
		return err
	}
	source, err := readFile(resolved)
	if err != nil {
		return fmt.Errorf("reading input %q: %w; verify the file is readable", opts.input, err)
	}
	if opts.output != "" {
		return writePersistent(opts.output, source, resolved, stdout)
	}
	return openTemporary(source, resolved)
}

func runCLI(args []string, stdout, stderr io.Writer) int {
	if err := run(args, stdout); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", programName, err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr))
}
