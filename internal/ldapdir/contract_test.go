//go:build ldapcontract

// Contract tests for the real LDAP adapter against a real OpenLDAP server.
// They are not part of the default test set. Run them with:
//
//	make test-ldap-contract        # = go test -tags ldapcontract ./internal/ldapdir
//
// By default a throwaway osixia/openldap container is started (unique name, dynamic
// host port, label xproj-test). To use an existing server instead, set
// XP_TEST_LDAP_URL, XP_TEST_LDAP_BASE_DN, XP_TEST_LDAP_ADMIN_DN and XP_TEST_LDAP_ADMIN_PASSWORD;
// the tests add and remove entries below ou=xp-contract,<base>.
package ldapdir_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/go-ldap/ldap/v3"

	"github.com/admax12959/X-Project/internal/domain"
	"github.com/admax12959/X-Project/internal/ldapdir"
)

const (
	image       = "osixia/openldap:1.5.0"
	adminPW     = "contract-admin-pw"
	defaultBase = "dc=corp,dc=example"
)

var server struct {
	url, base, adminDN, adminPW string
	peopleDN                    string
}

func TestMain(m *testing.M) {
	code := func() int {
		if u := os.Getenv("XP_TEST_LDAP_URL"); u != "" {
			server.url, server.base = u, os.Getenv("XP_TEST_LDAP_BASE_DN")
			server.adminDN, server.adminPW = os.Getenv("XP_TEST_LDAP_ADMIN_DN"), os.Getenv("XP_TEST_LDAP_ADMIN_PASSWORD")
		} else {
			stop, err := startContainer()
			if err != nil {
				fmt.Fprintln(os.Stderr, "ldap contract: start OpenLDAP:", err)
				return 1
			}
			defer stop()
		}
		if err := seed(); err != nil {
			fmt.Fprintln(os.Stderr, "ldap contract: seed:", err)
			return 1
		}
		return m.Run()
	}()
	os.Exit(code)
}

func startContainer() (stop func(), err error) {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	name := "xproj-test-ldap-" + hex.EncodeToString(b)
	out, err := exec.Command("docker", "run", "-d", "--rm",
		"--name", name, "--label", "xproj-test=1",
		"-e", "LDAP_DOMAIN=corp.example", "-e", "LDAP_ADMIN_PASSWORD="+adminPW, "-e", "LDAP_TLS=false",
		"-p", "127.0.0.1::389", image).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker run: %v: %s", err, out)
	}
	stop = func() { _ = exec.Command("docker", "rm", "-f", name).Run() }
	out, err = exec.Command("docker", "port", name, "389/tcp").Output()
	if err != nil {
		stop()
		return nil, fmt.Errorf("docker port: %v", err)
	}
	hostPort := strings.TrimSpace(strings.Split(strings.TrimSpace(string(out)), "\n")[0])
	server.url, server.base = "ldap://"+hostPort, defaultBase
	server.adminDN, server.adminPW = "cn=admin,"+defaultBase, adminPW

	deadline := time.Now().Add(3 * time.Minute)
	for {
		c, derr := ldap.DialURL(server.url)
		if derr == nil {
			derr = c.Bind(server.adminDN, server.adminPW)
			c.Close()
		}
		if derr == nil {
			return stop, nil
		}
		if time.Now().After(deadline) {
			stop()
			return nil, fmt.Errorf("OpenLDAP not ready: %v", derr)
		}
		time.Sleep(time.Second)
	}
}

func person(c *ldap.Conn, uid, cn, mail, dept, pw, employeeType, sn string) error {
	req := ldap.NewAddRequest("uid="+uid+","+server.peopleDN, nil)
	req.Attribute("objectClass", []string{"inetOrgPerson"})
	req.Attribute("uid", []string{uid})
	req.Attribute("cn", []string{cn})
	req.Attribute("sn", []string{sn})
	req.Attribute("displayName", []string{cn})
	req.Attribute("mail", []string{mail})
	req.Attribute("departmentNumber", []string{dept})
	req.Attribute("userPassword", []string{pw})
	if employeeType != "" {
		req.Attribute("employeeType", []string{employeeType})
	}
	return c.Add(req)
}

func seed() error {
	c, err := ldap.DialURL(server.url)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := c.Bind(server.adminDN, server.adminPW); err != nil {
		return err
	}
	server.peopleDN = "ou=xp-contract," + server.base
	ou := ldap.NewAddRequest(server.peopleDN, nil)
	ou.Attribute("objectClass", []string{"organizationalUnit"})
	ou.Attribute("ou", []string{"xp-contract"})
	if err := c.Add(ou); err != nil && !ldap.IsErrorWithCode(err, ldap.LDAPResultEntryAlreadyExists) {
		return err
	}
	for _, p := range [][]string{
		// uid, cn, mail, department, password, employeeType, sn
		{"elarsen", "Emma Larsen", "emma.larsen@corp.example", "Payments Platform", "emma-pw", "", "Larsen"},
		{"gone", "Gone Person", "gone@corp.example", "Ops", "gone-pw", "disabled", "Person"},
		{"twin1", "Twin One", "t1@corp.example", "Ops", "t1-pw", "", "twin"},
		{"twin2", "Twin Two", "t2@corp.example", "Ops", "t2-pw", "", "twin"},
	} {
		if err := person(c, p[0], p[1], p[2], p[3], p[4], p[5], p[6]); err != nil && !ldap.IsErrorWithCode(err, ldap.LDAPResultEntryAlreadyExists) {
			return err
		}
	}
	return nil
}

func newDir(t *testing.T, mutate ...func(*ldapdir.Config)) *ldapdir.Directory {
	t.Helper()
	cfg := ldapdir.Config{
		URL: server.url, BindDN: server.adminDN, BindPassword: server.adminPW, BaseDN: server.peopleDN,
		DeptAttr: "departmentNumber", DisabledFilter: "(employeeType=disabled)", Timeout: 5 * time.Second,
	}
	for _, m := range mutate {
		m(&cfg)
	}
	d, err := ldapdir.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestAuthenticateReturnsDirectoryAttributes(t *testing.T) {
	d := newDir(t)
	res := d.Authenticate(context.Background(), "elarsen", "emma-pw")
	want := domain.DirectoryEntry{Login: "elarsen", DisplayName: "Emma Larsen", Email: "emma.larsen@corp.example", Department: "Payments Platform"}
	if res.Outcome != domain.DirectoryOK || res.Entry != want {
		t.Fatalf("Authenticate = %+v, want OK %+v", res, want)
	}
}

func TestAuthenticateFailureKinds(t *testing.T) {
	d := newDir(t)
	ctx := context.Background()
	cases := []struct {
		name, login, pw string
		want            domain.DirectoryOutcome
	}{
		{"wrong password", "elarsen", "nope", domain.DirectoryBadPassword},
		{"empty password is never an anonymous success", "elarsen", "", domain.DirectoryBadPassword},
		{"unknown account", "nobody", "x", domain.DirectoryNotFound},
		{"disabled account", "gone", "gone-pw", domain.DirectoryDisabled},
		{"wildcard is not a pattern", "*", "emma-pw", domain.DirectoryNotFound},
		{"filter injection is escaped", "elarsen)(uid=*", "emma-pw", domain.DirectoryNotFound},
	}
	for _, c := range cases {
		if got := d.Authenticate(ctx, c.login, c.pw).Outcome; got != c.want {
			t.Errorf("%s: outcome = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestLookupNeedsNoPassword(t *testing.T) {
	d := newDir(t)
	ctx := context.Background()
	for login, want := range map[string]domain.DirectoryOutcome{
		"elarsen": domain.DirectoryOK, "gone": domain.DirectoryDisabled, "nobody": domain.DirectoryNotFound,
	} {
		if got := d.Lookup(ctx, login).Outcome; got != want {
			t.Errorf("Lookup(%q) = %d, want %d", login, got, want)
		}
	}
}

func TestUnreachableIsNeverNotFound(t *testing.T) {
	ctx := context.Background()
	cases := map[string]func(*ldapdir.Config){
		"nothing listens":          func(c *ldapdir.Config) { c.URL = "ldap://127.0.0.1:1" },
		"service account refused":  func(c *ldapdir.Config) { c.BindPassword = "wrong" },
		"base dn does not exist":   func(c *ldapdir.Config) { c.BaseDN = "ou=missing," + server.base },
		"user filter is ambiguous": func(c *ldapdir.Config) { c.UserFilter = "(|(uid=%s)(sn=twin))" },
	}
	for name, m := range cases {
		d := newDir(t, m)
		login := "elarsen"
		if name == "user filter is ambiguous" {
			login = "twin1"
		}
		if got := d.Lookup(ctx, login).Outcome; got != domain.DirectoryUnreachable {
			t.Errorf("%s: Lookup outcome = %d, want Unreachable", name, got)
		}
		if got := d.Authenticate(ctx, login, "emma-pw").Outcome; got != domain.DirectoryUnreachable {
			t.Errorf("%s: Authenticate outcome = %d, want Unreachable", name, got)
		}
	}
}
