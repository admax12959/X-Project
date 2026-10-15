package domain

import "slices"

// Roles and permissions (spec #18, docs/design/iam-scope.md).
//
// A Permission is a code-defined capability. A Role is an immutable set of
// permissions. The three roles below are built in: they cannot be created,
// edited or deleted, and they are granted to individual users only.
//
//	administrator  global; the user's IsAdmin flag
//	owner          scoped to one application; the application's Owner list
//	pe             scoped to one application; the application's PE list
//
// Every authorization check in the domain service asks "does the actor hold
// this permission?" through requirePermission / requireAppPermission; no rule
// compares role names. Keep the grants here equal to the permission matrix of
// spec #2.

// Permission identifies one capability, such as "line.fork".
type Permission string

const (
	PermAppCreate        Permission = "app.create"
	PermAppDelete        Permission = "app.delete"
	PermAppEdit          Permission = "app.edit"           // name, description, main branch
	PermAppMembersManage Permission = "app.members.manage" // Owner / PE lists (ADR-0003)
	PermAppDeployEdit    Permission = "app.deploy.edit"    // deployment info
	PermLineFork         Permission = "line.fork"
	PermLineEdit         Permission = "line.edit"
	PermLineStatusSet    Permission = "line.status.set"
	PermVersionRegister  Permission = "version.register"
	PermRepoVerify       Permission = "repo.verify"
	PermPlatformManage   Permission = "platform.manage"
	PermUserManage       Permission = "user.manage"
	PermRoleView         Permission = "role.view"
	PermAuditView        Permission = "audit.view"
	PermConfigManage     Permission = "config.manage" // system configuration: LDAP now, mail and storage later (ADR-0005)
)

// RoleScope says what a role is granted over.
type RoleScope string

const (
	ScopeGlobal      RoleScope = "global"
	ScopeApplication RoleScope = "application"
)

// RoleID identifies a built-in role.
type RoleID string

const (
	RoleAdministrator RoleID = "administrator"
	RoleOwnerID       RoleID = "owner"
	RolePEID          RoleID = "pe"
)

// Role is a built-in role: a named set of permissions with a scope.
type Role struct {
	ID          RoleID
	Scope       RoleScope
	Permissions []Permission
}

// builtInRoles is the whole role catalogue, in display order. Not in the database.
var builtInRoles = []Role{
	{ID: RoleAdministrator, Scope: ScopeGlobal, Permissions: []Permission{
		PermAppCreate, PermAppDelete, PermAppMembersManage,
		PermRepoVerify, PermPlatformManage, PermUserManage, PermRoleView, PermAuditView, PermConfigManage,
	}},
	{ID: RoleOwnerID, Scope: ScopeApplication, Permissions: []Permission{
		PermAppEdit, PermAppMembersManage, PermAppDeployEdit,
		PermLineFork, PermLineEdit, PermLineStatusSet, PermVersionRegister,
	}},
	{ID: RolePEID, Scope: ScopeApplication, Permissions: []Permission{
		PermAppDeployEdit,
	}},
}

func (r Role) has(p Permission) bool { return slices.Contains(r.Permissions, p) }

func builtInRole(id RoleID) Role {
	for _, r := range builtInRoles {
		if r.ID == id {
			return r
		}
	}
	panic("unknown built-in role " + string(id))
}

// BuiltInRoles lists the built-in roles with their permissions, for the read-only
// role page. Administrators only (role.view).
func (s *Service) BuiltInRoles(actor Actor) ([]Role, error) {
	if err := requirePermission(actor, PermRoleView); err != nil {
		return nil, err
	}
	out := make([]Role, len(builtInRoles))
	for i, r := range builtInRoles {
		r.Permissions = slices.Clone(r.Permissions)
		out[i] = r
	}
	return out, nil
}

// globalRoles are the roles the actor holds everywhere.
func globalRoles(actor Actor) []Role {
	if actor.User.IsAdmin {
		return []Role{builtInRole(RoleAdministrator)}
	}
	return nil
}

// applicationRoles are the roles the actor holds on one application.
func applicationRoles(actor Actor, app Application) []Role {
	var roles []Role
	if containsUser(app.Owners, actor.User.ID) {
		roles = append(roles, builtInRole(RoleOwnerID))
	}
	if containsUser(app.PEs, actor.User.ID) {
		roles = append(roles, builtInRole(RolePEID))
	}
	return roles
}

func anyHas(roles []Role, p Permission) bool {
	for _, r := range roles {
		if r.has(p) {
			return true
		}
	}
	return false
}

// requirePermission is the check for operations that are not about one application:
// the actor needs a global role that holds the permission.
func requirePermission(actor Actor, p Permission) error {
	if !anyHas(globalRoles(actor), p) {
		return forbidden()
	}
	return nil
}

// requireAppPermission is the check for operations on one application: a global role
// or a role the actor holds on that application must hold the permission.
func requireAppPermission(actor Actor, app Application, p Permission) error {
	if !anyHas(globalRoles(actor), p) && !anyHas(applicationRoles(actor, app), p) {
		return forbidden()
	}
	return nil
}
