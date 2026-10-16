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

// Every permission point declared in permissions.go, and every built-in role, must have
// a description in both frontend dictionaries ("perm.<key>" and "roles.role.<id>.*"),
// so the read-only role page never shows a bare key.
func TestEveryPermissionAndRoleHasAChineseAndEnglishDescription(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "permissions.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var perms []string
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			continue
		}
		for _, s := range g.Specs {
			vs := s.(*ast.ValueSpec)
			for i, n := range vs.Names {
				if strings.HasPrefix(n.Name, "Perm") {
					v, _ := strconv.Unquote(vs.Values[i].(*ast.BasicLit).Value)
					perms = append(perms, v)
				}
			}
		}
	}
	if len(perms) == 0 {
		t.Fatal("found no permission constants")
	}
	for _, lang := range []string{"zh", "en"} {
		dict, err := os.ReadFile("../../web/src/i18n/" + lang + "/roles.ts")
		if err != nil {
			t.Fatal(err)
		}
		have := map[string]bool{}
		for _, m := range regexp.MustCompile(`'([\w.]+)':`).FindAllStringSubmatch(string(dict), -1) {
			have[m[1]] = true
		}
		for _, p := range perms {
			if !have["perm."+p] {
				t.Errorf("permission %q has no 'perm.%s' key in web/src/i18n/%s/roles.ts", p, p, lang)
			}
		}
		for _, r := range builtInRoles {
			for _, k := range []string{"roles.role." + string(r.ID) + ".name", "roles.role." + string(r.ID) + ".desc"} {
				if !have[k] {
					t.Errorf("role %q has no '%s' key in web/src/i18n/%s/roles.ts", r.ID, k, lang)
				}
			}
		}
	}
	// Every permission is held by at least one built-in role (no dead permission points).
	for _, p := range perms {
		held := false
		for _, r := range builtInRoles {
			held = held || r.has(Permission(p))
		}
		if !held {
			t.Errorf("permission %q is granted to no built-in role", p)
		}
	}
}
