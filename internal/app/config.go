package app

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/admax12959/X-Project/internal/domain"
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
	// The master key encrypts every secret stored in the database (ADR-0005). It comes from
	// SecretKey (XP_SECRET_KEY) or from the file SecretKeyFile (XP_SECRET_KEY_FILE, a Docker
	// secret such as /run/secrets/xp_secret_key); exactly one of them is required.
	SecretKey        string   `yaml:"secret_key"`
	SecretKeyFile    string   `yaml:"secret_key_file"`
	SecretKeyVersion int      `yaml:"secret_key_version"` // XP_SECRET_KEY_VERSION, recorded with every stored secret; default 1
	Admin            struct { // initial administrator, used only on the very first start
		Username    string `yaml:"username"`     // XP_ADMIN_USERNAME
		Password    string `yaml:"password"`     // XP_ADMIN_PASSWORD
		DisplayName string `yaml:"display_name"` // XP_ADMIN_DISPLAY_NAME
		Email       string `yaml:"email"`        // XP_ADMIN_EMAIL
	} `yaml:"admin"`
	// LDAP is only read once: on a start whose database has no LDAP configuration yet it is
	// imported into the database, which is the source of truth from then on (ADR-0005; the
	// administrator edits it on the LDAP page). An empty url means there is nothing to import.
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
	// Mail is the delivery side of the outgoing mail queue (#20). The SMTP server itself is
	// configured by an administrator on the Email page, not here.
	Mail struct {
		PollInterval time.Duration   `yaml:"poll_interval"` // XP_MAIL_POLL_INTERVAL, how often the queue is looked at, default 5s
		RetryBackoff []time.Duration `yaml:"retry_backoff"` // XP_MAIL_RETRY_BACKOFF, comma separated waits after the 1st, 2nd, ... failed delivery; delivery is tried len+1 times. Default 1m,5m,30m,2h,6h
	} `yaml:"mail"`
	// Storage: until an administrator configures an S3-compatible object storage (admin page
	// "Storage"), objects (avatars) live in a directory on the local disk. Mount a volume here.
	Storage struct {
		LocalDir string `yaml:"local_dir"` // XP_STORAGE_DIR, default data/storage
	} `yaml:"storage"`
	Session struct {
		TTL         time.Duration `yaml:"ttl"`           // XP_SESSION_TTL, default 12h
		CookieSecure bool          `yaml:"cookie_secure"` // XP_COOKIE_SECURE, set true behind HTTPS
	} `yaml:"session"`
}

// DefaultConfig returns the defaults before file and environment are applied.
func DefaultConfig() Config {
	var c Config
	c.Listen = ":8080"
	c.Database.ConnectTimeout = 90 * time.Second
	c.Session.TTL = 12 * time.Hour
	c.SecretKeyVersion = 1
	c.Mail.PollInterval = 5 * time.Second
	c.Mail.RetryBackoff = append([]time.Duration(nil), domain.DefaultMailRetryBackoff...)
	c.Storage.LocalDir = "data/storage"
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
	str("XP_STORAGE_DIR", &c.Storage.LocalDir)
	str("XP_DB_DSN", &c.Database.DSN)
	str("XP_SECRET_KEY", &c.SecretKey)
	str("XP_SECRET_KEY_FILE", &c.SecretKeyFile)
	if v, ok := os.LookupEnv("XP_SECRET_KEY_VERSION"); ok {
		if c.SecretKeyVersion, err = strconv.Atoi(v); err != nil {
			err = fmt.Errorf("XP_SECRET_KEY_VERSION: %w", err)
		}
	}
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
	dur("XP_MAIL_POLL_INTERVAL", &c.Mail.PollInterval)
	if v, ok := os.LookupEnv("XP_MAIL_RETRY_BACKOFF"); ok && err == nil {
		c.Mail.RetryBackoff = nil
		for _, part := range strings.Split(v, ",") {
			if part = strings.TrimSpace(part); part == "" {
				continue
			}
			d, perr := time.ParseDuration(part)
			if perr != nil {
				err = fmt.Errorf("XP_MAIL_RETRY_BACKOFF: %w", perr)
				break
			}
			c.Mail.RetryBackoff = append(c.Mail.RetryBackoff, d)
		}
	}
	dur("XP_SESSION_TTL", &c.Session.TTL)
	boolean("XP_COOKIE_SECURE", &c.Session.CookieSecure)
	if err != nil {
		return c, err
	}
	if c.Database.DSN == "" {
		return c, fmt.Errorf("database DSN is required (database.dsn or XP_DB_DSN)")
	}
	if c.Mail.PollInterval <= 0 {
		return c, fmt.Errorf("mail.poll_interval must be positive")
	}
	if _, err := c.MasterKey(); err != nil {
		return c, err
	}
	return c, nil
}

// MasterKey returns the master key: the configured value, or the content of the key file
// (a Docker secret) without its trailing line break. Exactly one source must be set; a
// missing source, two sources, an unreadable file and an empty file are all errors that say
// what to do.
func (c Config) MasterKey() (string, error) {
	const hint = "set the master key with XP_SECRET_KEY (secret_key) or, for a Docker secret, with XP_SECRET_KEY_FILE (secret_key_file)"
	switch {
	case c.SecretKey != "" && c.SecretKeyFile != "":
		return "", fmt.Errorf("master key: both a key and a key file are set; %s, not both", hint)
	case c.SecretKey != "":
		return c.SecretKey, nil
	case c.SecretKeyFile == "":
		return "", fmt.Errorf("master key is required: %s", hint)
	}
	b, err := os.ReadFile(c.SecretKeyFile)
	if err != nil {
		return "", fmt.Errorf("master key file: %w", err)
	}
	key := strings.TrimRight(string(b), "\r\n")
	if key == "" {
		return "", fmt.Errorf("master key file %s is empty", c.SecretKeyFile)
	}
	return key, nil
}
