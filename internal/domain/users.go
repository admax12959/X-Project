package domain

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// UserWithIdentities is a user together with every login identity attached to it.
type UserWithIdentities struct {
	User User
	// SecondFactorEnabled: the user has a second factor turned on (#23). Read from the
	// repository with the user, so every administrator view of a user carries it.
	SecondFactorEnabled bool
	Identities          []LoginIdentity
}

// UserRepository is the user-management part of Repository.
type UserRepository interface {
	// ListUsers returns every user (oldest first) with its login identities.
	ListUsers(ctx context.Context) ([]UserWithIdentities, error)
	// ListUsersInDepartments is ListUsers limited to users whose department is one of ids.
	ListUsersInDepartments(ctx context.Context, ids []DepartmentID) ([]UserWithIdentities, error)
	// LockUserWithIdentities reads one user and its identities, locking the user
	// row for the rest of the transaction (call inside InTx).
	LockUserWithIdentities(ctx context.Context, id UserID) (UserWithIdentities, bool, error)
	// GetUserWithIdentities reads one user and its identities without locking.
	GetUserWithIdentities(ctx context.Context, id UserID) (UserWithIdentities, bool, error)
	// UpdateUser overwrites the editable columns (username, display name, email,
	// phone, language). Returns ErrUsernameTaken on a username conflict.
	UpdateUser(ctx context.Context, u User, at time.Time) error
	// RenameEmailLogin changes the login email of the user's email identity.
	// Returns ErrIdentityTaken on a conflict.
	RenameEmailLogin(ctx context.Context, id UserID, loginEmail string) error
	// ChangePassword replaces the password hash of the user's email identity and clears
	// its must-change flag. It leaves the sessions alone: the caller decides which ones
	// survive (ClearSessionMustChangePassword, DeleteOtherUserSessions, DeleteUserSessions).
	ChangePassword(ctx context.Context, id UserID, hash string) error
	// SetTemporaryPassword replaces the password hash of the user's email identity and marks it
	// as one the user must change at the next login (the command line, #23).
	SetTemporaryPassword(ctx context.Context, id UserID, hash string) error
	SetUserStatus(ctx context.Context, id UserID, status UserStatus, at time.Time) error
	// SetUserAdmin sets the administrator flag.
	SetUserAdmin(ctx context.Context, id UserID, isAdmin bool, at time.Time) error
	// LockEnabledAdmins locks the rows of every enabled administrator, in ID order, for the
	// rest of the transaction and returns their IDs. Call inside InTx, before locking a user.
	LockEnabledAdmins(ctx context.Context) ([]UserID, error)
	// ReadUsersLocked reads the given users (those that exist) under a shared lock, taken
	// in ID order, so that none of them can be disabled until the transaction ends.
	ReadUsersLocked(ctx context.Context, ids []UserID) (map[UserID]User, error)
}

// ListUsers returns all users for the administrator's user list, oldest first. A non-empty
// departmentID limits it to the users in that department or any department below it
// (422 invalid on departmentId when it is malformed or unknown).
func (s *Service) ListUsers(ctx context.Context, actor Actor, departmentID string) ([]UserWithIdentities, error) {
	if err := requirePermission(actor, PermUserManage); err != nil {
		return nil, err
	}
	return s.listUsers(ctx, departmentID)
}

func (s *Service) listUsers(ctx context.Context, departmentID string) ([]UserWithIdentities, error) {
	ids, filtered, err := s.departmentFilter(ctx, departmentID)
	if err != nil {
		return nil, err
	}
	if filtered {
		return s.repo.ListUsersInDepartments(ctx, ids)
	}
	return s.repo.ListUsers(ctx)
}

// CreateEmailUserInput is the administrator's request to create a user with an email
// identity. The login email is also the user's contact email. Password is a temporary
// one the user must change at the first login; it is used only while mail is not configured.
// With mail configured the user is invited instead and Password is ignored (#21).
type CreateEmailUserInput struct {
	Username     string
	DisplayName  string
	LoginEmail   string
	Phone        string
	DepartmentID string // optional: the department to put the user in
	Password     string
}

// CreateEmailUser creates a user with an email identity bound by the administrator. The
// administrator typed the login email, so it counts as verified. It is the single entry
// point for creating users that do not come from the directory.
//
// When mail is configured the user is created without a password and an invitation mail
// with a one-time link is queued in the same transaction; the user chooses the password on
// the set-password page. Without mail the administrator's temporary password is used.
func (s *Service) CreateEmailUser(ctx context.Context, actor Actor, in CreateEmailUserInput) (UserWithIdentities, error) {
	if err := requirePermission(actor, PermUserManage); err != nil {
		return UserWithIdentities{}, err
	}
	var c Collector
	in.Username = strings.TrimSpace(in.Username)
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	in.LoginEmail = normalizeLoginEmail(in.LoginEmail)
	in.Phone = strings.TrimSpace(in.Phone)
	checkUsername(&c, in.Username)
	checkLoginEmail(&c, "loginEmail", in.LoginEmail)
	checkProfile(&c, in.DisplayName, "", in.Phone)
	invite, err := s.MailConfigured(ctx)
	if err != nil {
		return UserWithIdentities{}, err
	}
	if !invite {
		checkNewPassword(&c, in.Password)
	}
	departmentID := parseDepartmentRef(&c, "departmentId", in.DepartmentID)
	if err := c.Err(); err != nil {
		return UserWithIdentities{}, err
	}

	// ADR-0004: LDAP owns the namespace. Asked before the transaction (a network call);
	// an unreachable directory fails the creation, the check is never skipped.
	if err := s.CheckUsernameFreeInDirectory(ctx, "username", in.Username); err != nil {
		return UserWithIdentities{}, err
	}
	hash := "" // an invited user has no password until the link is used
	if !invite {
		if hash, err = hashPassword(in.Password); err != nil {
			return UserWithIdentities{}, err
		}
	}
	var created UserWithIdentities
	err = s.repo.InTx(ctx, func(tx Repository) error {
		now := s.nowUTC()
		u := User{
			Username: in.Username, DisplayName: in.DisplayName, Email: in.LoginEmail,
			Phone: in.Phone, Status: UserEnabled, CreatedAt: now,
		}
		var dept Department
		if departmentID != 0 {
			var found bool
			if dept, found, err = tx.GetDepartment(ctx, departmentID); err != nil {
				return err
			} else if !found {
				return Validation(Invalid("departmentId", CodeInvalid, nil))
			}
			u.DepartmentID = departmentID
		}
		id, err := tx.CreateUser(ctx, u)
		if errors.Is(err, ErrUsernameTaken) {
			return usernameTaken()
		}
		if errors.Is(err, ErrDepartmentMissing) {
			return Validation(Invalid("departmentId", CodeInvalid, nil))
		}
		if err != nil {
			return err
		}
		u.ID = id
		ident := LoginIdentity{
			UserID: id, Kind: IdentityEmail, LoginName: in.LoginEmail,
			PasswordHash: hash, PasswordSet: hash != "", BoundAt: now, BoundVia: BoundViaAdmin, MustChangePassword: !invite,
		}
		created = UserWithIdentities{User: u, Identities: []LoginIdentity{ident}}
		err = tx.CreateIdentity(ctx, ident)
		if errors.Is(err, ErrIdentityTaken) {
			return loginEmailTaken("loginEmail")
		}
		if err != nil {
			return err
		}
		if err := s.recordAudit(ctx, tx, AuditActorOf(actor), userCreatedAudit(id, in.LoginEmail)); err != nil {
			return err
		}
		if invite {
			if err := s.inviteWithin(ctx, tx, id, in.LoginEmail); err != nil {
				return err
			}
		}
		if departmentID != 0 {
			return s.recordAudit(ctx, tx, AuditActorOf(actor), userDepartmentChangedAudit(id, nil, departmentRef(dept)))
		}
		return nil
	})
	return created, err
}

// UpdateUserInput is an administrator's edit of a user. A nil field is left
// unchanged; a non-nil field is validated like it is at creation.
type UpdateUserInput struct {
	Username    *string
	DisplayName *string
	Email       *string // the contact email: see applyProfileEdit
	LoginEmail  *string // the login email of the user's email identity
	Phone       *string
	Department  OptionalRef // the department in the organization: unset = unchanged, empty ID = none
	IsAdmin     *bool       // grants or revokes the administrator role; the last enabled administrator cannot lose it
}

func (u UserWithIdentities) hasIdentity(kind IdentityKind) bool {
	for _, i := range u.Identities {
		if i.Kind == kind {
			return true
		}
	}
	return false
}

func (u UserWithIdentities) identity(kind IdentityKind) (LoginIdentity, bool) {
	for _, i := range u.Identities {
		if i.Kind == kind {
			return i, true
		}
	}
	return LoginIdentity{}, false
}

func trimmed(p *string) (string, bool) {
	if p == nil {
		return "", false
	}
	return strings.TrimSpace(*p), true
}

// profileEdit is an edit of the profile fields; a nil field is left unchanged.
type profileEdit struct {
	DisplayName *string
	Email       *string
	Phone       *string
}

// applyProfileEdit applies e to u and validates the fields that are being changed,
// reporting problems on c. For LDAP-bound users the directory owns display name and email
// (they are synced at login, without local validation), so a different
// value is refused with ldapManaged and an unchanged one is neither applied nor
// validated: a directory value that does not satisfy the local rules must not block the
// fields the user does own. For users without an LDAP identity the contact email is
// their login email, so a different value is refused with emailManaged: the login email
// is what changes. Phone is always editable. Used for both the administrator's edit of
// a user and a user's edit of themselves.
func applyProfileEdit(c *Collector, u User, ldapBound bool, e profileEdit) User {
	if v, ok := trimmed(e.DisplayName); ok {
		if ldapBound {
			if v != u.DisplayName {
				c.Add("displayName", CodeLdapManaged, nil)
			}
		} else {
			checkDisplayName(c, v)
			u.DisplayName = v
		}
	}
	if v, ok := trimmed(e.Email); ok && v != u.Email {
		if ldapBound {
			c.Add("email", CodeLdapManaged, nil)
		} else {
			c.Add("email", CodeEmailManaged, nil)
		}
	}
	if v, ok := trimmed(e.Phone); ok {
		checkLength(c, "phone", v, maxPhoneLength)
		u.Phone = v
	}
	return u
}

// UpdateUser edits a user's profile, the username for users without an LDAP identity
// (ADR-0004), and the login email. Everything refers to the user by ID, so a rename
// keeps all records. An administrator-entered login email counts as verified; for a user
// without LDAP it is the contact email too.
func (s *Service) UpdateUser(ctx context.Context, actor Actor, id UserID, in UpdateUserInput) (UserWithIdentities, error) {
	if err := requirePermission(actor, PermUserManage); err != nil {
		return UserWithIdentities{}, err
	}
	// ADR-0004: a new local login name must not exist in the directory. Like at creation,
	// the directory is asked before the transaction (a network call) and an unreachable
	// directory fails the rename; the check is never skipped. The transaction below
	// re-reads the user under its lock and makes the final decision about the rename.
	if v, ok := trimmed(in.Username); ok && usernamePattern.MatchString(v) {
		pre, found, err := s.repo.GetUserWithIdentities(ctx, id)
		if err != nil {
			return UserWithIdentities{}, err
		}
		if found && v != pre.User.Username && !pre.hasIdentity(IdentityLDAP) {
			if err := s.CheckUsernameFreeInDirectory(ctx, "username", v); err != nil {
				return UserWithIdentities{}, err
			}
		}
	}
	var result UserWithIdentities
	err := s.repo.InTx(ctx, func(tx Repository) error {
		// Revoking the administrator role: the administrators are locked first (in ID order)
		// so that two of them revoking each other at the same time cannot both succeed.
		var admins []UserID
		if in.IsAdmin != nil && !*in.IsAdmin {
			var err error
			if admins, err = tx.LockEnabledAdmins(ctx); err != nil {
				return err
			}
		}
		cur, found, err := tx.LockUserWithIdentities(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return newKind(KindNotFound, CodeNotFound)
		}
		u := cur.User
		contactBefore := cur.User.Email
		ldapBound := cur.hasIdentity(IdentityLDAP)
		var c Collector

		if v, ok := trimmed(in.Username); ok && v != u.Username {
			if ldapBound {
				c.Add("username", CodeUsernameLocked, nil)
			} else {
				checkUsername(&c, v)
				u.Username = v
			}
		}
		if in.IsAdmin != nil && *in.IsAdmin != u.IsAdmin && !*in.IsAdmin && u.Status == UserEnabled && len(admins) <= 1 {
			c.Add("isAdmin", CodeLastAdmin, nil)
		}
		newLoginEmail, oldLoginEmail := "", ""
		if in.LoginEmail != nil {
			v := normalizeLoginEmail(*in.LoginEmail)
			if !cur.hasIdentity(IdentityEmail) {
				c.Add("loginEmail", CodeNoEmailIdentity, nil)
			} else {
				checkLoginEmail(&c, "loginEmail", v)
				if ident, _ := cur.identity(IdentityEmail); v != ident.LoginName {
					newLoginEmail, oldLoginEmail = v, ident.LoginName
				}
			}
		}
		u = applyProfileEdit(&c, u, ldapBound, profileEdit{
			DisplayName: in.DisplayName, Email: in.Email, Phone: in.Phone,
		})
		newDept := u.DepartmentID
		if in.Department.Set {
			newDept = parseDepartmentRef(&c, "departmentId", in.Department.ID)
		}
		if err := c.Err(); err != nil {
			return err
		}
		// The department in the organization (not the directory department): any user may have
		// one, LDAP-bound or not. A foreign key keeps a department that is being deleted at
		// the same moment from receiving the user.
		var oldDeptRef, newDeptRef any
		if newDept != u.DepartmentID {
			if u.DepartmentID != 0 {
				old, _, err := tx.GetDepartment(ctx, u.DepartmentID)
				if err != nil {
					return err
				}
				oldDeptRef = departmentRef(old)
			}
			if newDept != 0 {
				d, found, err := tx.GetDepartment(ctx, newDept)
				if err != nil {
					return err
				}
				if !found {
					return Validation(Invalid("departmentId", CodeInvalid, nil))
				}
				newDeptRef = departmentRef(d)
			}
		}
		departmentChanged := newDept != u.DepartmentID
		u.DepartmentID = newDept

		if newLoginEmail != "" && !ldapBound {
			u.Email = newLoginEmail
		}
		if err := tx.UpdateUser(ctx, u, s.nowUTC()); errors.Is(err, ErrDepartmentMissing) {
			return Validation(Invalid("departmentId", CodeInvalid, nil))
		} else if err != nil {
			return mapUsernameTaken(err)
		}
		if departmentChanged {
			if err := s.recordAudit(ctx, tx, AuditActorOf(actor), userDepartmentChangedAudit(id, oldDeptRef, newDeptRef)); err != nil {
				return err
			}
		}
		if newLoginEmail != "" {
			if err := tx.RenameEmailLogin(ctx, id, newLoginEmail); errors.Is(err, ErrIdentityTaken) {
				return loginEmailTaken("loginEmail")
			} else if err != nil {
				return err
			}
			// A link mailed to the old address must not work any more.
			if err := tx.UseUserAuthTokens(ctx, id, passwordTokenPurposes, s.nowUTC()); err != nil {
				return err
			}
			for i := range cur.Identities {
				if cur.Identities[i].Kind == IdentityEmail {
					cur.Identities[i].LoginName = newLoginEmail
				}
			}
			if err := s.recordAudit(ctx, tx, AuditActorOf(actor), loginEmailChangedAudit(id, oldLoginEmail, newLoginEmail)); err != nil {
				return err
			}
			// Same notice as when the user changes it themselves: the old login address and, when
			// it is another one, the contact email as it was. No secret goes into it.
			data := map[string]string{"OldEmail": oldLoginEmail, "NewEmail": newLoginEmail}
			if err := s.notifySecurity(ctx, tx, id, oldLoginEmail, MailTemplateLoginEmailChanged, data); err != nil {
				return err
			}
			if contactBefore != oldLoginEmail {
				if err := s.notifySecurity(ctx, tx, id, contactBefore, MailTemplateLoginEmailChanged, data); err != nil {
					return err
				}
			}
		}
		if in.IsAdmin != nil && *in.IsAdmin != u.IsAdmin {
			if err := tx.SetUserAdmin(ctx, id, *in.IsAdmin, s.nowUTC()); err != nil {
				return err
			}
			if err := s.recordAudit(ctx, tx, AuditActorOf(actor), adminFlagChangedAudit(id, u.IsAdmin, *in.IsAdmin)); err != nil {
				return err
			}
			u.IsAdmin = *in.IsAdmin
		}
		result = UserWithIdentities{User: u, Identities: cur.Identities, SecondFactorEnabled: cur.SecondFactorEnabled}
		return nil
	})
	return result, err
}

// DisableUser blocks a user from logging in; open sessions stop working too
// (Authenticate rejects disabled users). Users are never deleted. Idempotent.
// A user who is the only enabled Owner of any application cannot be disabled
// (422 soleOwner with the blocking applications): an administrator hands the
// applications over first (ADR-0003).
func (s *Service) DisableUser(ctx context.Context, actor Actor, id UserID) (UserWithIdentities, error) {
	return s.setUserStatus(ctx, actor, id, UserDisabled)
}

// EnableUser lets a disabled user log in again. Idempotent.
func (s *Service) EnableUser(ctx context.Context, actor Actor, id UserID) (UserWithIdentities, error) {
	return s.setUserStatus(ctx, actor, id, UserEnabled)
}

func (s *Service) setUserStatus(ctx context.Context, actor Actor, id UserID, status UserStatus) (UserWithIdentities, error) {
	if err := requirePermission(actor, PermUserManage); err != nil {
		return UserWithIdentities{}, err
	}
	var result UserWithIdentities
	err := s.repo.InTx(ctx, func(tx Repository) error {
		cur, found, err := tx.LockUserWithIdentities(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return newKind(KindNotFound, CodeNotFound)
		}
		if cur.User.Status != status {
			if status == UserDisabled {
				if err := s.checkNotSoleOwner(ctx, tx, id); err != nil {
					return err
				}
			}
			if err := tx.SetUserStatus(ctx, id, status, s.nowUTC()); err != nil {
				return err
			}
			cur.User.Status = status
		}
		result = cur
		return nil
	})
	return result, err
}

func mapUsernameTaken(err error) error {
	if errors.Is(err, ErrUsernameTaken) {
		return usernameTaken()
	}
	return err
}

// Field limits, matching the users table.
const (
	minPasswordLength = 8
	maxPasswordBytes  = 72 // bcrypt ignores or rejects anything longer
	maxUsernameLength = 32
	maxNameLength     = 200 // display name and department name
	maxEmailLength    = 320
	maxPhoneLength    = 64
)

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// ErrUsernameTaken is returned by the Repository when the unique key on the username
// is violated.
var ErrUsernameTaken = errors.New("username already taken")

// ErrIdentityTaken is returned by the Repository when the unique key on (kind, login
// name) of the login identities is violated.
var ErrIdentityTaken = errors.New("login identity already taken")

func usernameTaken() error {
	return Validation(Invalid("username", CodeUsernameTaken, nil))
}

func loginEmailTaken(field string) error {
	return Validation(Invalid(field, CodeLoginEmailTaken, nil))
}

// normalizeLoginEmail is the stored form of a login email: trimmed and lower case, so
// that the uniqueness and the lookup do not depend on the case somebody typed.
func normalizeLoginEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// checkLoginEmail validates a login email (already normalized): required and shaped like an address.
func checkLoginEmail(c *Collector, field, email string) {
	if email == "" {
		c.Add(field, CodeRequired, nil)
	} else if !emailPattern.MatchString(email) {
		c.Add(field, CodeInvalid, nil)
	} else {
		checkLength(c, field, email, maxEmailLength)
	}
}

// checkUsername validates the username format (spec #2: ^[a-z][a-z0-9._-]{1,31}$).
func checkUsername(c *Collector, username string) {
	if username == "" {
		c.Add("username", CodeRequired, nil)
	} else if !usernamePattern.MatchString(username) {
		c.Add("username", CodeInvalid, nil)
	}
}

// checkNewPassword validates a password being set for an email identity.
func checkNewPassword(c *Collector, password string) { checkNewPasswordField(c, "password", password) }

func checkNewPasswordField(c *Collector, field, password string) {
	if password == "" {
		c.Add(field, CodeRequired, nil)
	} else if utf8.RuneCountInString(password) < minPasswordLength {
		c.Add(field, CodeTooShort, map[string]any{"min": minPasswordLength})
	} else if len(password) > maxPasswordBytes {
		c.Add(field, CodeTooLong, map[string]any{"max": maxPasswordBytes})
	}
}

func checkLength(c *Collector, field, value string, max int) {
	if utf8.RuneCountInString(value) > max {
		c.Add(field, CodeTooLong, map[string]any{"max": max})
	}
}

func checkDisplayName(c *Collector, displayName string) {
	if displayName == "" {
		c.Add("displayName", CodeRequired, nil)
	} else {
		checkLength(c, "displayName", displayName, maxNameLength)
	}
}

// checkEmail: optional, shaped like an address.
func checkEmail(c *Collector, email string) {
	if email == "" {
		return
	}
	if !emailPattern.MatchString(email) {
		c.Add("email", CodeInvalid, nil)
	} else {
		checkLength(c, "email", email, maxEmailLength)
	}
}

// checkProfile validates a whole profile at creation: display name (required), email
// (optional, shaped like an address) and phone. Values are already trimmed.
func checkProfile(c *Collector, displayName, email, phone string) {
	checkDisplayName(c, displayName)
	checkEmail(c, email)
	checkLength(c, "phone", phone, maxPhoneLength)
}
