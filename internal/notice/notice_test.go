package notice

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/queone/gkit/update"
)

// proxy serves version as the gkit @latest reply and counts requests; an
// empty version answers 500.
func proxy(t *testing.T, version string) (*httptest.Server, *int) {
	t.Helper()
	n := new(int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*n++
		if version == "" {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, `{"Version":%q}`, version)
	}))
	t.Cleanup(srv.Close)
	return srv, n
}

func options(t *testing.T, srv *httptest.Server) update.Options {
	t.Helper()
	t.Setenv("NO_UPDATE_CHECK", "")
	return update.Options{
		Module:    module,
		Current:   "v1.0.0",
		CacheFile: filepath.Join(t.TempDir(), "update.json"),
		Client:    srv.Client(),
		Proxy:     srv.URL,
	}
}

func TestShowPrintsTheHintForTheBuildKind(t *testing.T) {
	for _, tc := range []struct{ mainVersion, hint string }{
		{"(devel)", cloneHint},
		{"", cloneHint},
		{"v0.92.1-0.20261028120000-e947a26aaaaa+dirty", cloneHint},
		{"v0.92.0", installHint},
	} {
		srv, _ := proxy(t, "v9.9.9")
		var out bytes.Buffer
		show(&out, true, tc.mainVersion, options(t, srv))
		want := "gkit v9.9.9 is out (this build is v1.0.0): " + tc.hint + "\n"
		if out.String() != want {
			t.Errorf("main version %q: printed %q, want %q", tc.mainVersion, out.String(), want)
		}
	}
}

func TestShowPrintsNothingWithoutAnUpdate(t *testing.T) {
	for _, tc := range []struct{ name, proxyVersion, noCheck string }{
		{"same version", "v1.0.0", ""},
		{"older version", "v0.9.0", ""},
		{"proxy fails", "", ""},
		{"check turned off", "v9.9.9", "1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := proxy(t, tc.proxyVersion)
			o := options(t, srv)
			t.Setenv("NO_UPDATE_CHECK", tc.noCheck)
			var out bytes.Buffer
			show(&out, true, "(devel)", o)
			if out.Len() != 0 {
				t.Fatalf("printed %q, want nothing", out.String())
			}
		})
	}
}

func TestShowSkipsWithoutATerminal(t *testing.T) {
	srv, n := proxy(t, "v9.9.9")
	var out bytes.Buffer
	show(&out, false, "(devel)", options(t, srv))
	if out.Len() != 0 || *n != 0 {
		t.Fatalf("printed %q after %d requests, want nothing and no request", out.String(), *n)
	}
}

func TestDefaultOptionsShareOneGkitCache(t *testing.T) {
	o, err := defaultOptions()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := update.CacheDir("gkit")
	if err != nil {
		t.Fatal(err)
	}
	if o.Module != "github.com/queone/gkit" || o.CacheFile != filepath.Join(dir, "update.json") {
		t.Fatalf("options %+v, want the gkit module and %s", o, filepath.Join(dir, "update.json"))
	}
}

func TestEveryUtilityShowsTheNoticeFirst(t *testing.T) {
	paths, err := filepath.Glob("../../cmd/*/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("found no cmd/*/main.go")
	}
	for _, path := range paths {
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		if !mainStartsWithShow(f) {
			t.Errorf("%s: main does not call notice.Show() first", path)
		}
	}
}

func mainStartsWithShow(f *ast.File) bool {
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Name.Name != "main" || fn.Body == nil || len(fn.Body.List) == 0 {
			continue
		}
		es, ok := fn.Body.List[0].(*ast.ExprStmt)
		if !ok {
			return false
		}
		call, ok := es.X.(*ast.CallExpr)
		if !ok || len(call.Args) != 0 {
			return false
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		pkg, ok := sel.X.(*ast.Ident)
		return ok && pkg.Name == "notice" && sel.Sel.Name == "Show"
	}
	return false
}
