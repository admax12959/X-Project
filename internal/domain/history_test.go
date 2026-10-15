package domain

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Every EventType declared in history.go must have a "history.event.<type>" message
// in the frontend dictionaries (the i18n check keeps en and zh identical, so en
// suffices) and a renderer entry in web/src/history/render.ts.
func TestEveryEventTypeHasFrontendMessageAndRenderer(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "history.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			continue
		}
		for _, s := range g.Specs {
			vs := s.(*ast.ValueSpec)
			for i, n := range vs.Names {
				if strings.HasPrefix(n.Name, "Event") {
					v, _ := strconv.Unquote(vs.Values[i].(*ast.BasicLit).Value)
					types = append(types, v)
				}
			}
		}
	}
	if len(types) == 0 {
		t.Fatal("no event types found in history.go")
	}
	dict, err := os.ReadFile("../../web/src/i18n/en/history.ts")
	if err != nil {
		t.Fatal(err)
	}
	renderer, err := os.ReadFile("../../web/src/history/render.ts")
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, m := range regexp.MustCompile(`'history\.event\.(\w+)'`).FindAllStringSubmatch(string(dict), -1) {
		have[m[1]] = true
	}
	for _, typ := range types {
		if !have[typ] {
			t.Errorf("event type %q has no 'history.event.%s' key in web/src/i18n/en/history.ts", typ, typ)
		}
		if !regexp.MustCompile(`\b` + typ + `\s*:`).Match(renderer) {
			t.Errorf("event type %q has no renderer entry in web/src/history/render.ts", typ)
		}
	}
}
