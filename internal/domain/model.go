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
	// DirectoryDepartment is the department text synced from LDAP (read-only; GLOSSARY: 目录部门).
	// It has nothing to do with the organization's departments.
	DirectoryDepartment string
	// DepartmentID is the department the user belongs to in the organization; 0 = none.
	DepartmentID DepartmentID
	Language     string // LanguageZh, LanguageEn, or empty when never chosen
	Status       UserStatus
	IsAdmin      bool
	CreatedAt    time.Time
}

type IdentityKind string

const (
	IdentityEmail IdentityKind = "email" // login email + password
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
	LoginName    string // LDAP: the directory login name; email: the login email, lower case
	PasswordHash string // email only: bcrypt (salted); empty for LDAP and for an invited user who has not chosen a password. Loaded only where a password is checked.
	// PasswordSet (email only) is whether the identity has a password. Unlike PasswordHash it is
	// loaded everywhere, including the user lists; false means the user was invited and the link
	// has not been used yet.
	PasswordSet bool
	BoundAt     time.Time
	BoundVia    BoundVia
	// MustChangePassword (email only) is set while the password is one an administrator
	// chose: a session opened with it can do nothing but change the password and log out.
	MustChangePassword bool
}

// Session is a logged-in browser session. Only the token's hash is stored.
type Session struct {
	TokenHash          string
	UserID             UserID
	CreatedAt          time.Time
	ExpiresAt          time.Time
	MustChangePassword bool // opened with an administrator-chosen password
}

// Repository is the persistence port. Only the domain service uses it;
// the MySQL implementation lives in internal/store/mysqlstore.
type Repository interface {
	IdentityRepository
	CodePlatformRepository
	AuditRepository
	DepartmentRepository
	SystemConfigRepository
	MailOutboxRepository
	AuthTokenRepository

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

	ApplicationRepository
	VersionRepository
	LineRepository
	LineLifecycleRepository
	GlobalVersionRepository
	HistoryRepository
	ActivityRepository
	ApplicationMaintenanceRepository
}
