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

// Every error code declared in codes.go must have an "error.<code>" message in the
// frontend dictionaries (the i18n check keeps en and zh identical, so en suffices).
func TestEveryErrorCodeHasAFrontendMessage(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "codes.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			continue
		}
		for _, s := range g.Specs {
			for i, n := range s.(*ast.ValueSpec).Names {
				if strings.HasPrefix(n.Name, "Code") {
					v, _ := strconv.Unquote(s.(*ast.ValueSpec).Values[i].(*ast.BasicLit).Value)
					codes = append(codes, v)
				}
			}
		}
	}
	dict, err := os.ReadFile("../../web/src/i18n/en/errors.ts")
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, m := range regexp.MustCompile(`'error\.(\w+)'`).FindAllStringSubmatch(string(dict), -1) {
		have[m[1]] = true
	}
	for _, c := range codes {
		if !have[c] {
			t.Errorf("error code %q has no 'error.%s' key in web/src/i18n/en/errors.ts", c, c)
		}
	}
}
