package app

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the whole runtime configuration. Load reads an optional YAML file
// (XP_CONFIG) and then applies environment variables, which win.
// Every field and variable is documented in docs/dev.md.
type Config struct {
	Listen   string `yaml:"listen"`  // XP_LISTEN, default ":8080"
	WebDir   string `yaml:"web_dir"` // XP_WEB_DIR, built frontend; empty = API only
	Database struct {
		DSN            string        `yaml:"dsn"`             // XP_DB_DSN, go-sql-driver DSN, e.g. user:pw@tcp(host:3306)/xproject
		ConnectTimeout time.Duration `yaml:"connect_timeout"` // XP_DB_CONNECT_TIMEOUT, default 90s
	} `yaml:"database"`
	SecretKey string `yaml:"secret_key"` // XP_SECRET_KEY, required: encrypts stored secrets (code platform tokens); keep it stable
	Admin     struct { // initial administrator, used only on the very first start
		Username    string `yaml:"username"`     // XP_ADMIN_USERNAME
		Password    string `yaml:"password"`     // XP_ADMIN_PASSWORD
		DisplayName string `yaml:"display_name"` // XP_ADMIN_DISPLAY_NAME
		Email       string `yaml:"email"`        // XP_ADMIN_EMAIL
	} `yaml:"admin"`
	// LDAP is the one corporate directory. Leaving url empty means LDAP login is not configured.
	LDAP struct {
		URL            string        `yaml:"url"`             // XP_LDAP_URL, ldap://host:389 or ldaps://host:636
		StartTLS       bool          `yaml:"start_tls"`       // XP_LDAP_START_TLS
		SkipVerify     bool          `yaml:"skip_verify"`     // XP_LDAP_SKIP_VERIFY, accept any server certificate
		BindDN         string        `yaml:"bind_dn"`         // XP_LDAP_BIND_DN, service account for searching; empty = anonymous
		BindPassword   string        `yaml:"bind_password"`   // XP_LDAP_BIND_PASSWORD
		BaseDN         string        `yaml:"base_dn"`         // XP_LDAP_BASE_DN, users are searched below it
		UserFilter     string        `yaml:"user_filter"`     // XP_LDAP_USER_FILTER, one %s for the login name, default (uid=%s)
		LoginAttr      string        `yaml:"login_attr"`      // XP_LDAP_LOGIN_ATTR, default uid
		NameAttr       string        `yaml:"name_attr"`       // XP_LDAP_NAME_ATTR, default displayName
		EmailAttr      string        `yaml:"email_attr"`      // XP_LDAP_EMAIL_ATTR, default mail
		DepartmentAttr string        `yaml:"department_attr"` // XP_LDAP_DEPARTMENT_ATTR, default department
		DisabledFilter string        `yaml:"disabled_filter"` // XP_LDAP_DISABLED_FILTER, matches an account when disabled; empty = never disabled
		Timeout        time.Duration `yaml:"timeout"`         // XP_LDAP_TIMEOUT, default 10s
	} `yaml:"ldap"`
	Session struct {
		TTL          time.Duration `yaml:"ttl"`           // XP_SESSION_TTL, default 12h
		CookieSecure bool          `yaml:"cookie_secure"` // XP_COOKIE_SECURE, set true behind HTTPS
	} `yaml:"session"`
}

// DefaultConfig returns the defaults before file and environment are applied.
func DefaultConfig() Config {
	var c Config
	c.Listen = ":8080"
	c.Database.ConnectTimeout = 90 * time.Second
	c.Session.TTL = 12 * time.Hour
	return c
}

// Load builds the configuration from defaults, the YAML file named by XP_CONFIG
// (if set), and environment variables.
func Load() (Config, error) {
	c := DefaultConfig()
	if path := os.Getenv("XP_CONFIG"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return c, fmt.Errorf("read config: %w", err)
		}
		if err := yaml.Unmarshal(b, &c); err != nil {
			return c, fmt.Errorf("parse config %s: %w", path, err)
		}
	}
	str := func(name string, dst *string) {
		if v, ok := os.LookupEnv(name); ok {
			*dst = v
		}
	}
	var err error
	dur := func(name string, dst *time.Duration) {
		if v, ok := os.LookupEnv(name); ok && err == nil {
			if *dst, err = time.ParseDuration(v); err != nil {
				err = fmt.Errorf("%s: %w", name, err)
			}
		}
	}
	boolean := func(name string, dst *bool) {
		if v, ok := os.LookupEnv(name); ok && err == nil {
			if *dst, err = strconv.ParseBool(v); err != nil {
				err = fmt.Errorf("%s: %w", name, err)
			}
		}
	}
	str("XP_LISTEN", &c.Listen)
	str("XP_WEB_DIR", &c.WebDir)
	str("XP_DB_DSN", &c.Database.DSN)
	str("XP_SECRET_KEY", &c.SecretKey)
	dur("XP_DB_CONNECT_TIMEOUT", &c.Database.ConnectTimeout)
	str("XP_LDAP_URL", &c.LDAP.URL)
	boolean("XP_LDAP_START_TLS", &c.LDAP.StartTLS)
	boolean("XP_LDAP_SKIP_VERIFY", &c.LDAP.SkipVerify)
	str("XP_LDAP_BIND_DN", &c.LDAP.BindDN)
	str("XP_LDAP_BIND_PASSWORD", &c.LDAP.BindPassword)
	str("XP_LDAP_BASE_DN", &c.LDAP.BaseDN)
	str("XP_LDAP_USER_FILTER", &c.LDAP.UserFilter)
	str("XP_LDAP_LOGIN_ATTR", &c.LDAP.LoginAttr)
	str("XP_LDAP_NAME_ATTR", &c.LDAP.NameAttr)
	str("XP_LDAP_EMAIL_ATTR", &c.LDAP.EmailAttr)
	str("XP_LDAP_DEPARTMENT_ATTR", &c.LDAP.DepartmentAttr)
	str("XP_LDAP_DISABLED_FILTER", &c.LDAP.DisabledFilter)
	dur("XP_LDAP_TIMEOUT", &c.LDAP.Timeout)
	str("XP_ADMIN_USERNAME", &c.Admin.Username)
	str("XP_ADMIN_PASSWORD", &c.Admin.Password)
	str("XP_ADMIN_DISPLAY_NAME", &c.Admin.DisplayName)
	str("XP_ADMIN_EMAIL", &c.Admin.Email)
	dur("XP_SESSION_TTL", &c.Session.TTL)
	boolean("XP_COOKIE_SECURE", &c.Session.CookieSecure)
	if err != nil {
		return c, err
	}
	if c.Database.DSN == "" {
		return c, fmt.Errorf("database DSN is required (database.dsn or XP_DB_DSN)")
	}
	if c.SecretKey == "" {
		return c, fmt.Errorf("secret key is required (secret_key or XP_SECRET_KEY)")
	}
	return c, nil
}
