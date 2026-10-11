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
