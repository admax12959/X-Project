package app

import "testing"

func TestNewDirectoryIsNilWhenLDAPIsNotConfigured(t *testing.T) {
	d, err := NewDirectory(DefaultConfig())
	if err != nil || d != nil {
		t.Fatalf("NewDirectory(default) = %v, %v; want nil, nil", d, err)
	}
}

func TestNewDirectoryRejectsPartialConfiguration(t *testing.T) {
	cases := map[string]func(*Config){
		"base without url":   func(c *Config) { c.LDAP.BaseDN = "dc=corp" },
		"url without base":   func(c *Config) { c.LDAP.URL = "ldap://x:389" },
		"bind dn without pw": func(c *Config) { c.LDAP.URL, c.LDAP.BaseDN, c.LDAP.BindDN = "ldap://x:389", "dc=corp", "cn=svc" },
		"filter without %s":  func(c *Config) { c.LDAP.URL, c.LDAP.BaseDN, c.LDAP.UserFilter = "ldap://x:389", "dc=corp", "(uid=x)" },
		"filter with two %s": func(c *Config) {
			c.LDAP.URL, c.LDAP.BaseDN, c.LDAP.UserFilter = "ldap://x:389", "dc=corp", "(|(uid=%s)(cn=%s))"
		},
	}
	for name, mutate := range cases {
		cfg := DefaultConfig()
		mutate(&cfg)
		if d, err := NewDirectory(cfg); err == nil {
			t.Errorf("%s: NewDirectory = %v, nil; want an error", name, d)
		}
	}
}

func TestNewDirectoryBuildsTheAdapterWhenFullyConfigured(t *testing.T) {
	cfg := DefaultConfig()
	cfg.LDAP.URL, cfg.LDAP.BaseDN = "ldap://127.0.0.1:1", "dc=corp,dc=example"
	d, err := NewDirectory(cfg) // does not connect
	if err != nil || d == nil {
		t.Fatalf("NewDirectory = %v, %v; want an adapter", d, err)
	}
}
