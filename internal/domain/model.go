package domain

import (
	"context"
	"time"
)

// UserID identifies a user. All internal references (Owner/PE, change history,
// login identities) use the ID, never the username (ADR-0004).
type UserID int64

type UserStatus string

const (
	UserEnabled  UserStatus = "enabled"
	UserDisabled UserStatus = "disabled"
)

// User is one natural person (GLOSSARY: 用户).
type User struct {
	ID          UserID
	Username    string
	DisplayName string
	Email       string
	Phone       string
	Department  string
	Status      UserStatus
	IsAdmin     bool
	CreatedAt   time.Time
}

type IdentityKind string

const (
	IdentityLocal IdentityKind = "local"
	IdentityLDAP  IdentityKind = "ldap"
)

// BoundVia records how a login identity got attached to its user (spec story 8).
type BoundVia string

const (
	BoundViaFirstLogin BoundVia = "firstLogin" // created on first LDAP login
	BoundViaSelf       BoundVia = "self"       // user bound it in settings
	BoundViaAdmin      BoundVia = "admin"      // an administrator bound it
	BoundViaBootstrap  BoundVia = "bootstrap"  // initial administrator on first start
)

// LoginIdentity is one way a user can log in (GLOSSARY: 登录身份).
// At most one identity per kind per user.
type LoginIdentity struct {
	ID           int64
	UserID       UserID
	Kind         IdentityKind
	LoginName    string
	PasswordHash string // local only: bcrypt (salted); empty for LDAP
	BoundAt      time.Time
	BoundVia     BoundVia
}

// Session is a logged-in browser session. Only the token's hash is stored.
type Session struct {
	TokenHash string
	UserID    UserID
	CreatedAt time.Time
	ExpiresAt time.Time
}

// ApplicationSummary is one row of the application list.
type ApplicationSummary struct {
	Key         string
	Name        string
	Description string
}

// Repository is the persistence port. Only the domain service uses it;
// the MySQL implementation lives in internal/store/mysqlstore.
type Repository interface {
	CodePlatformRepository

	// InTx runs fn in one transaction; fn receives a Repository bound to it.
	InTx(ctx context.Context, fn func(tx Repository) error) error

	// LockBootstrap locks the single bootstrap marker row (inside InTx) and reports
	// whether first-start initialisation has already happened.
	LockBootstrap(ctx context.Context) (done bool, err error)
	MarkBootstrapped(ctx context.Context, at time.Time) error

	CreateUser(ctx context.Context, u User) (UserID, error)
	GetUser(ctx context.Context, id UserID) (User, bool, error)
	CreateIdentity(ctx context.Context, i LoginIdentity) error
	FindIdentity(ctx context.Context, kind IdentityKind, loginName string) (LoginIdentity, bool, error)
	UserRepository

	CreateSession(ctx context.Context, s Session) error
	FindSession(ctx context.Context, tokenHash string) (Session, bool, error)
	DeleteSession(ctx context.Context, tokenHash string) error

	ListApplications(ctx context.Context) ([]ApplicationSummary, error)
}
