package domain

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// Audit log (spec #18, docs/design/iam-scope.md "审计"): an append-only record of
// identity- and security-related operations, shown to administrators on the audit log
// page. Like the change history (history.go) an entry is an action TYPE plus structured
// PARAMS; the frontend renders the sentence in the current UI language
// (web/src/audit/render.ts and the audit.event.* messages). docs/dev.md explains how to
// add an action.
//
// Rules:
//   - The entry is written in the same transaction as the change it records
//     (recordAudit(ctx, tx, by, ev) with the tx Repository).
//   - The object is one target: a user (target id = user ID) or an application (target
//     id = application key). Applications can be deleted and audit entries survive, so
//     params of application events carry the application as {key, name}.
//   - References to users inside params are userParam(id), resolved to names on read.
//   - Params never contain secrets: for configuration changes record WHICH fields changed,
//     not their values.

// AuditAction identifies what happened. The value is the wire format and the suffix of
// the frontend message key "audit.event.<action>".
type AuditAction string

const (
	// AuditUserCreated: an administrator created a user. Target: the user. Params: loginEmail.
	AuditUserCreated AuditAction = "userCreated"
	// AuditLoginEmailChanged: the login email of a user's email identity changed. Target:
	// the user. Params: from, to.
	AuditLoginEmailChanged AuditAction = "loginEmailChanged"
	// AuditIdentityBound: a login identity was attached to a user. Target: the user.
	// Params: kind = "ldap" | "email", loginName, via = "self" | "admin".
	AuditIdentityBound AuditAction = "identityBound"
	// AuditIdentityUnbound: a login identity was removed from a user. Target: the user.
	// Params: kind, loginName.
	AuditIdentityUnbound AuditAction = "identityUnbound"
	// AuditAppMemberAdded / AuditAppMemberRemoved: one person added to or removed from the
	// Owner or PE list of an application. Target: the application. Params: role = "owner" |
	// "pe", user = userParam, application = {key, name}.
	AuditAppMemberAdded   AuditAction = "appMemberAdded"
	AuditAppMemberRemoved AuditAction = "appMemberRemoved"
	// AuditAdminFlagChanged: the administrator flag of a user changed. Target: the user.
	// Params: from, to (booleans).
	AuditAdminFlagChanged AuditAction = "adminFlagChanged"
	// AuditDepartmentCreated: a department was added to the organization. Target: the
	// department. Params: name, parent = {id, name} or null for a top-level department.
	AuditDepartmentCreated AuditAction = "departmentCreated"
	// AuditDepartmentRenamed: Target: the department. Params: from, to (names).
	AuditDepartmentRenamed AuditAction = "departmentRenamed"
	// AuditDepartmentMoved: a department moved under another one or to the top level.
	// Target: the department. Params: name, from, to = {id, name} or null (top level).
	AuditDepartmentMoved AuditAction = "departmentMoved"
	// AuditDepartmentDeleted: Target: the department (gone). Params: name, parent = {id, name} or null.
	AuditDepartmentDeleted AuditAction = "departmentDeleted"
	// AuditUserDepartmentChanged: a user was put into, moved between or taken out of
	// departments. Target: the user. Params: from, to = {id, name} or null (no department).
	AuditUserDepartmentChanged AuditAction = "userDepartmentChanged"
	// AuditConfigChanged: a section of the system configuration was saved with changes
	// (#19). Target: the section (type systemConfig, id "ldap", ...). Params: section,
	// changes = the NAMES of the changed fields (a secret is listed by name, never its value).
	AuditConfigChanged AuditAction = "configChanged"
	// AuditSecondFactorEnabled: a user turned on a second-factor method for their own login
	// (#22). Target: the user. Params: type = "totp". Never the secret.
	AuditSecondFactorEnabled AuditAction = "secondFactorEnabled"
	// AuditSecondFactorDisabled: a user turned their second factor off (#31). Target: the user.
	// Params: none.
	AuditSecondFactorDisabled AuditAction = "secondFactorDisabled"
	// AuditRecoveryCodesRegenerated: a user made a new set of recovery codes; the old set is
	// void (#31). Target: the user. Params: none. Never the codes.
	AuditRecoveryCodesRegenerated AuditAction = "recoveryCodesRegenerated"
	// AuditSecondFactorReset: an administrator or the command line removed a user's second
	// factor, for example after the phone was lost (#23). Target: the user. Params: none. The
	// operator is the entry's actor.
	AuditSecondFactorReset AuditAction = "secondFactorReset"
	// AuditPasswordReset: the command line gave a user a new temporary password (#23); the
	// user has to change it at the next login. Target: the user. Params: none. Never the password.
	AuditPasswordReset AuditAction = "passwordReset"
)

// AuditTargetType says what kind of object an entry is about.
type AuditTargetType string

const (
	AuditTargetUser        AuditTargetType = "user"
	AuditTargetApplication AuditTargetType = "application"
	// AuditTargetDepartment: the id is the department ID. The department may be deleted
	// later; its entries stay, and their params carry its name.
	AuditTargetDepartment AuditTargetType = "department"
	// AuditTargetSystemConfig: a section of the system configuration; the id is the section name.
	AuditTargetSystemConfig AuditTargetType = "systemConfig"
)

// AuditActor is who performed an operation: a user, or the command line in the container.
type AuditActor struct {
	CLI    bool
	UserID UserID // set unless CLI
}

// AuditActorCLI is the actor of operations done through the command line.
var AuditActorCLI = AuditActor{CLI: true}

// AuditActorOf is the audit actor for an authenticated request.
func AuditActorOf(a Actor) AuditActor { return AuditActor{UserID: a.User.ID} }

// auditEvent is an audit entry that has not been written yet. Build it with one
// constructor per action so params stay consistent.
type auditEvent struct {
	action     AuditAction
	targetType AuditTargetType
	targetID   string
	params     map[string]any
}

func userTargetID(id UserID) string { return strconv.FormatInt(int64(id), 10) }

func userCreatedAudit(id UserID, loginEmail string) auditEvent {
	return auditEvent{AuditUserCreated, AuditTargetUser, userTargetID(id), map[string]any{"loginEmail": loginEmail}}
}

func loginEmailChangedAudit(id UserID, from, to string) auditEvent {
	return auditEvent{AuditLoginEmailChanged, AuditTargetUser, userTargetID(id), map[string]any{"from": from, "to": to}}
}

func identityBoundAudit(id UserID, kind IdentityKind, loginName string, via BoundVia) auditEvent {
	return auditEvent{AuditIdentityBound, AuditTargetUser, userTargetID(id),
		map[string]any{"kind": string(kind), "loginName": loginName, "via": string(via)}}
}

func identityUnboundAudit(id UserID, kind IdentityKind, loginName string) auditEvent {
	return auditEvent{AuditIdentityUnbound, AuditTargetUser, userTargetID(id),
		map[string]any{"kind": string(kind), "loginName": loginName}}
}

func appMemberAudit(action AuditAction, app Application, role MemberRole, user UserID) auditEvent {
	return auditEvent{action, AuditTargetApplication, app.Key, map[string]any{
		"role": string(role), "user": userParam(user), "application": map[string]any{"key": app.Key, "name": app.Name},
	}}
}

func adminFlagChangedAudit(id UserID, from, to bool) auditEvent {
	return auditEvent{AuditAdminFlagChanged, AuditTargetUser, userTargetID(id), map[string]any{"from": from, "to": to}}
}

func secondFactorEnabledAudit(id UserID, t AuthenticatorType) auditEvent {
	return auditEvent{AuditSecondFactorEnabled, AuditTargetUser, userTargetID(id), map[string]any{"type": string(t)}}
}

func secondFactorDisabledAudit(id UserID) auditEvent {
	return auditEvent{AuditSecondFactorDisabled, AuditTargetUser, userTargetID(id), map[string]any{}}
}

func recoveryCodesRegeneratedAudit(id UserID) auditEvent {
	return auditEvent{AuditRecoveryCodesRegenerated, AuditTargetUser, userTargetID(id), map[string]any{}}
}

func secondFactorResetAudit(id UserID) auditEvent {
	return auditEvent{AuditSecondFactorReset, AuditTargetUser, userTargetID(id), map[string]any{}}
}

func passwordResetAudit(id UserID) auditEvent {
	return auditEvent{AuditPasswordReset, AuditTargetUser, userTargetID(id), map[string]any{}}
}

func departmentTargetID(id DepartmentID) string { return strconv.FormatInt(int64(id), 10) }

func departmentCreatedAudit(d Department, parent any) auditEvent {
	return auditEvent{AuditDepartmentCreated, AuditTargetDepartment, departmentTargetID(d.ID), map[string]any{"name": d.Name, "parent": parent}}
}

func departmentRenamedAudit(d Department, to string) auditEvent {
	return auditEvent{AuditDepartmentRenamed, AuditTargetDepartment, departmentTargetID(d.ID), map[string]any{"from": d.Name, "to": to}}
}

func departmentMovedAudit(d Department, from, to any) auditEvent {
	return auditEvent{AuditDepartmentMoved, AuditTargetDepartment, departmentTargetID(d.ID), map[string]any{"name": d.Name, "from": from, "to": to}}
}

func departmentDeletedAudit(d Department, parent any) auditEvent {
	return auditEvent{AuditDepartmentDeleted, AuditTargetDepartment, departmentTargetID(d.ID), map[string]any{"name": d.Name, "parent": parent}}
}

func userDepartmentChangedAudit(id UserID, from, to any) auditEvent {
	return auditEvent{AuditUserDepartmentChanged, AuditTargetUser, userTargetID(id), map[string]any{"from": from, "to": to}}
}

func configChangedAudit(section string, changes []string) auditEvent {
	return auditEvent{AuditConfigChanged, AuditTargetSystemConfig, section, map[string]any{"section": section, "changes": changes}}
}

// StoredAuditEntry is an audit row as the repository keeps it.
type StoredAuditEntry struct {
	ID         int64
	At         time.Time
	ActorCLI   bool
	ActorID    UserID // 0 when ActorCLI
	Action     AuditAction
	TargetType AuditTargetType
	TargetID   string
	Params     []byte // JSON object
}

// AuditFilter narrows the audit log. Zero fields match everything.
type AuditFilter struct {
	ActorCLI   bool   // only entries of the command line
	ActorID    UserID // only entries of this user
	TargetType AuditTargetType
	TargetID   string
	Action     AuditAction
	From, To   time.Time // inclusive bounds on the time; zero = unbounded
}

// AuditRepository is the audit part of Repository. There is deliberately no update and
// no delete: the audit log is append-only.
type AuditRepository interface {
	AppendAudit(ctx context.Context, e StoredAuditEntry) error
	// ListAudit returns one page (offset, limit) of the entries that match the filter,
	// newest first (occurred_at, then ID), and the number of matches in all pages.
	ListAudit(ctx context.Context, f AuditFilter, offset, limit int) (items []StoredAuditEntry, total int, err error)
}

// recordAudit appends ev to the audit log on behalf of by. Call it inside the same InTx
// as the change it describes, with the tx Repository.
func (s *Service) recordAudit(ctx context.Context, tx Repository, by AuditActor, ev auditEvent) error {
	params, err := json.Marshal(ev.params)
	if err != nil {
		return err
	}
	return tx.AppendAudit(ctx, StoredAuditEntry{
		At: s.nowUTC(), ActorCLI: by.CLI, ActorID: by.UserID,
		Action: ev.action, TargetType: ev.targetType, TargetID: ev.targetID, Params: params,
	})
}

// AuditActorRef is the operator of an entry as displayed: the command line, or a user.
type AuditActorRef struct {
	CLI  bool
	User UserRef
}

// AuditTarget is the object of an entry as displayed. User is set for user targets.
type AuditTarget struct {
	Type AuditTargetType
	ID   string
	User *UserRef
}

// AuditEntry is an audit entry ready for display. Params is the stored params with every
// user reference enriched: {"userId", "username", "displayName"}.
type AuditEntry struct {
	ID     int64
	At     time.Time
	Actor  AuditActorRef
	Action AuditAction
	Target AuditTarget
	Params map[string]any
}

// AuditQuery is the request as sent: every field is the raw query string. Actor is a
// user ID or "cli"; From and To are RFC 3339 instants (both inclusive); Page and PageSize
// as in the global version list (empty = page 1, 20 per page; at most 100).
type AuditQuery struct {
	Actor, TargetType, TargetID, Action, From, To string
	Page, PageSize                                string
}

// AuditPage is one page of the audit log. Total counts all matches.
type AuditPage struct {
	Items    []AuditEntry
	Total    int
	Page     int
	PageSize int
}

const (
	defaultAuditPageSize = 20
	maxAuditPageSize     = 100
)

// ListAudit returns one page of the audit log, newest first. Administrators only
// (audit.view). A page past the end is empty.
func (s *Service) ListAudit(ctx context.Context, actor Actor, q AuditQuery) (AuditPage, error) {
	if err := requirePermission(actor, PermAuditView); err != nil {
		return AuditPage{}, err
	}
	var c Collector
	f := AuditFilter{TargetID: strings.TrimSpace(q.TargetID), Action: AuditAction(strings.TrimSpace(q.Action))}
	switch a := strings.TrimSpace(q.Actor); {
	case a == "":
	case a == "cli":
		f.ActorCLI = true
	default:
		n, err := strconv.ParseInt(a, 10, 64)
		if err != nil || n <= 0 {
			c.Add("actor", CodeInvalid, nil)
		}
		f.ActorID = UserID(n)
	}
	switch tt := AuditTargetType(strings.TrimSpace(q.TargetType)); tt {
	case "", AuditTargetUser, AuditTargetApplication, AuditTargetDepartment, AuditTargetSystemConfig:
		f.TargetType = tt
	default:
		c.Add("targetType", CodeInvalid, nil)
	}
	f.From = parseInstant(&c, "from", q.From)
	f.To = parseInstant(&c, "to", q.To)
	page := positiveInt(&c, "page", q.Page, 1, 1<<31-1)
	size := positiveInt(&c, "pageSize", q.PageSize, defaultAuditPageSize, maxAuditPageSize)
	if err := c.Err(); err != nil {
		return AuditPage{}, err
	}

	stored, total, err := s.repo.ListAudit(ctx, f, (page-1)*size, size)
	if err != nil {
		return AuditPage{}, err
	}
	users := userResolver{repo: s.repo, cache: map[UserID]UserRef{}}
	out := AuditPage{Items: make([]AuditEntry, 0, len(stored)), Total: total, Page: page, PageSize: size}
	for _, e := range stored {
		entry, err := users.auditEntry(ctx, e)
		if err != nil {
			return AuditPage{}, err
		}
		out.Items = append(out.Items, entry)
	}
	return out, nil
}

// parseInstant reads an optional RFC 3339 instant; blank is the zero time.
func parseInstant(c *Collector, field, raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		c.Add(field, CodeInvalid, nil)
	}
	return t.UTC()
}

func (r userResolver) auditEntry(ctx context.Context, e StoredAuditEntry) (AuditEntry, error) {
	var params map[string]any
	if err := json.Unmarshal(e.Params, &params); err != nil {
		return AuditEntry{}, err
	}
	resolved, err := r.enrich(ctx, params)
	if err != nil {
		return AuditEntry{}, err
	}
	out := AuditEntry{ID: e.ID, At: e.At, Action: e.Action, Params: resolved.(map[string]any),
		Actor: AuditActorRef{CLI: e.ActorCLI}, Target: AuditTarget{Type: e.TargetType, ID: e.TargetID}}
	if !e.ActorCLI {
		if out.Actor.User, err = r.ref(ctx, e.ActorID); err != nil {
			return AuditEntry{}, err
		}
	}
	if e.TargetType == AuditTargetUser {
		if id, perr := strconv.ParseInt(e.TargetID, 10, 64); perr == nil {
			ref, err := r.ref(ctx, UserID(id))
			if err != nil {
				return AuditEntry{}, err
			}
			out.Target.User = &ref
		}
	}
	return out, nil
}
