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

// Every AuditAction declared in audit.go must have an "audit.event.<action>" sentence and an
// "audit.action.<action>" name in both frontend dictionaries, and a renderer entry in
// web/src/audit/render.ts, so the audit page never shows a bare key or "unknown action".
func TestEveryAuditActionHasFrontendMessagesAndRenderer(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "audit.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var actions []string
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			continue
		}
		for _, s := range g.Specs {
			vs := s.(*ast.ValueSpec)
			for i, n := range vs.Names {
				if strings.HasPrefix(n.Name, "Audit") && !strings.HasPrefix(n.Name, "AuditTarget") {
					v, _ := strconv.Unquote(vs.Values[i].(*ast.BasicLit).Value)
					actions = append(actions, v)
				}
			}
		}
	}
	if len(actions) == 0 {
		t.Fatal("no audit actions found in audit.go")
	}
	renderer, err := os.ReadFile("../../web/src/audit/render.ts")
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"zh", "en"} {
		dict, err := os.ReadFile("../../web/src/i18n/" + lang + "/audit.ts")
		if err != nil {
			t.Fatal(err)
		}
		have := map[string]bool{}
		for _, m := range regexp.MustCompile(`'(audit\.(?:event|action)\.\w+)'`).FindAllStringSubmatch(string(dict), -1) {
			have[m[1]] = true
		}
		for _, a := range actions {
			for _, key := range []string{"audit.event." + a, "audit.action." + a} {
				if !have[key] {
					t.Errorf("audit action %q has no '%s' key in web/src/i18n/%s/audit.ts", a, key, lang)
				}
			}
		}
	}
	for _, a := range actions {
		if !regexp.MustCompile(`\b` + a + `\s*:`).Match(renderer) {
			t.Errorf("audit action %q has no renderer entry in web/src/audit/render.ts", a)
		}
	}
}
