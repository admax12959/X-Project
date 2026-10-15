package domain

import (
	"context"
	"time"
)

// ---------------------------------------------------------------- Code platform (ADR-0001)

// PlatformKind is the type of a code platform.
type PlatformKind string

const (
	PlatformGitLab PlatformKind = "gitlab"
	PlatformGitHub PlatformKind = "github"
)

// PlatformConn is what an adapter needs to call one code platform instance.
type PlatformConn struct {
	Kind    PlatformKind
	BaseURL string
	Token   string // read-only service account token, decrypted
}

// RepoOutcome is the result kind of resolving a repository path.
type RepoOutcome int

const (
	RepoFound           RepoOutcome = iota + 1
	RepoNotFound                    // path does not exist on the platform
	RepoForbidden                   // the token has no access
	PlatformUnreachable             // network error, timeout, 5xx
)

// RepoResult carries the platform's stable project ID when Outcome is RepoFound.
type RepoResult struct {
	Outcome   RepoOutcome
	ProjectID string
}

// CodePlatformAdapter resolves a repository path to the platform's stable project ID.
// One implementation per PlatformKind; the domain picks by PlatformConn.Kind.
type CodePlatformAdapter interface {
	ResolveRepo(ctx context.Context, platform PlatformConn, path string) RepoResult
}

// ---------------------------------------------------------------- Directory / LDAP (ADR-0002, ADR-0004)

// DirectoryEntry is the attributes the directory owns for an account.
type DirectoryEntry struct {
	Login       string
	DisplayName string
	Email       string
	Department  string
}

// DirectoryOutcome is the result kind of a directory call.
type DirectoryOutcome int

const (
	DirectoryOK          DirectoryOutcome = iota + 1
	DirectoryBadPassword                  // Authenticate only: account exists, password wrong
	DirectoryNotFound                     // no such login name
	DirectoryDisabled                     // account exists but is disabled
	DirectoryUnreachable                  // directory could not be queried; never treat as NotFound
)

// DirectoryResult carries the account's attributes when Outcome is DirectoryOK.
type DirectoryResult struct {
	Outcome DirectoryOutcome
	Entry   DirectoryEntry
}

// LDAPConfig is everything needed to talk to the directory, with the service account
// password decrypted. It is built from the stored configuration (ldap_config.go) every
// time the directory is needed, and from the unsaved form for "test connection".
type LDAPConfig struct {
	URL            string
	StartTLS       bool
	SkipVerify     bool
	BindDN         string
	BindPassword   string
	BaseDN         string
	UserFilter     string
	LoginAttr      string
	NameAttr       string
	EmailAttr      string
	DepartmentAttr string
	DisabledFilter string
	Timeout        time.Duration
}

// DirectoryFactory creates a Directory for a configuration. It does not connect. A nil
// factory in Deps means LDAP is not available at all.
type DirectoryFactory interface {
	Build(cfg LDAPConfig) (Directory, error)
}

// Directory is the LDAP directory a configuration points to (see DirectoryFactory).
type Directory interface {
	// Authenticate checks login + password and returns the account's attributes.
	Authenticate(ctx context.Context, login, password string) DirectoryResult
	// Lookup reports whether login currently names a valid account (no password).
	Lookup(ctx context.Context, login string) DirectoryResult
}
