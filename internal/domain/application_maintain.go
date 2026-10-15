package domain

import (
	"context"
	"errors"
	"slices"
	"strings"
)

// Application maintenance (spec #2 permission matrix):
//
//	basic info (name, description, main branch)  Owner only
//	Owner/PE lists                               administrator (ADR-0003) or Owner
//	deployment info                              Owner or PE
//
// The application key and the repository binding have no edit path: they are not
// part of any input type here. Every change is recorded in the change history in
// the same transaction.

// ApplicationMaintenanceRepository is the maintenance part of Repository.
type ApplicationMaintenanceRepository interface {
	// LockApplication locks the application row for the rest of the transaction.
	LockApplication(ctx context.Context, id int64) error
	// UpdateApplicationInfo overwrites name and description.
	UpdateApplicationInfo(ctx context.Context, id int64, name, description string) error
	// LockOwnedApplications locks every application the user is an Owner of (ordered by
	// key, to keep lock order stable) and reads, under a shared lock, how many other
	// enabled Owners each has. Call inside InTx.
	LockOwnedApplications(ctx context.Context, userID UserID) ([]OwnedApplication, error)
	// DeleteApplication deletes the application row; lines, versions, members and
	// change events go with it through ON DELETE CASCADE.
	DeleteApplication(ctx context.Context, id int64) error
	SetDeployInfo(ctx context.Context, id int64, text string) error
	// SetMainBranch renames the branch on the application's mainline row.
	SetMainBranch(ctx context.Context, applicationID int64, branch string) error
}

func containsUser(list []UserRef, id UserID) bool {
	for _, u := range list {
		if u.ID == id {
			return true
		}
	}
	return false
}

func forbidden() error { return newKind(KindForbidden, CodeForbidden) }

// UpdateApplicationInput is the Owner's edit of the basic information. A nil
// field is left unchanged.
type UpdateApplicationInput struct {
	Name        *string
	Description *string
	MainBranch  *string
}

// UpdateApplication edits the basic information. Owners only: neither
// administrators nor PEs may change it.
func (s *Service) UpdateApplication(ctx context.Context, actor Actor, key string, in UpdateApplicationInput) (Application, error) {
	var result Application
	err := s.repo.InTx(ctx, func(tx Repository) error {
		app, err := s.lockApplicationByKey(ctx, tx, key)
		if err != nil {
			return err
		}
		if err := requireAppPermission(actor, app, PermAppEdit); err != nil {
			return err
		}
		name, description, branch := app.Name, app.Description, app.MainBranch
		var c Collector
		if in.Name != nil {
			name = strings.TrimSpace(*in.Name)
			if c.Required("name", name) {
				checkLength(&c, "name", name, maxNameLength)
			}
		}
		if in.Description != nil {
			description = strings.TrimSpace(*in.Description)
			checkLength(&c, "description", description, maxDescriptionLength)
		}
		if in.MainBranch != nil {
			branch = strings.TrimSpace(*in.MainBranch)
			if c.Required("mainBranch", branch) && !validBranchName(branch) {
				c.Add("mainBranch", CodeInvalid, nil)
			}
		}
		if err := c.Err(); err != nil {
			return err
		}
		changes := map[string]any{}
		if name != app.Name {
			changes["name"] = map[string]any{"from": app.Name, "to": name}
		}
		if description != app.Description {
			changes["description"] = map[string]any{"from": app.Description, "to": description}
		}
		if branch != app.MainBranch {
			changes["mainBranch"] = map[string]any{"from": app.MainBranch, "to": branch}
		}
		if len(changes) > 0 {
			if err := tx.UpdateApplicationInfo(ctx, app.ID, name, description); err != nil {
				return err
			}
			if err := tx.SetMainBranch(ctx, app.ID, branch); errors.Is(err, ErrBranchTaken) {
				return s.branchTakenError(ctx, tx, app.ID, "mainBranch", branch)
			} else if err != nil {
				return err
			}
			if err := s.recordEvent(ctx, tx, app.ID, actor, appInfoChangedEvent(changes)); err != nil {
				return err
			}
		}
		result, _, err = tx.GetApplicationByKey(ctx, key)
		return err
	})
	return result, err
}

// UpdateMembersInput replaces both lists. Both must be present: an Owner list
// that is missing or empty is rejected, the PE list may be empty.
type UpdateMembersInput struct {
	Owners *[]string
	PEs    *[]string
}

// UpdateMembers replaces the Owner and PE lists. Administrators may do it for any
// application (ADR-0003), Owners for their own; PEs may not. The application keeps
// at least one Owner. Anyone newly added must be an existing enabled user; a
// person who is already in a list may stay in it even if disabled since. One
// change-history event is written per person added or removed, with the real actor.
func (s *Service) UpdateMembers(ctx context.Context, actor Actor, key string, in UpdateMembersInput) (Application, error) {
	var result Application
	err := s.repo.InTx(ctx, func(tx Repository) error {
		// Parse first: the people named are locked before the application row (see lockMembers).
		var c Collector
		var owners, pes []UserID
		if in.Owners == nil || len(*in.Owners) == 0 {
			c.Add("owners", CodeRequired, nil)
		} else {
			owners = parseUserIDs(&c, "owners", *in.Owners)
		}
		if in.PEs == nil {
			c.Add("pes", CodeRequired, nil)
		} else {
			pes = parseUserIDs(&c, "pes", *in.PEs)
		}
		if _, found, err := tx.GetApplicationByKey(ctx, key); err != nil {
			return err
		} else if !found {
			return newKind(KindNotFound, CodeNotFound)
		}
		users, err := s.lockMembers(ctx, tx, owners, pes)
		if err != nil {
			return err
		}
		app, err := s.lockApplicationByKey(ctx, tx, key)
		if err != nil {
			return err
		}
		if err := requireAppPermission(actor, app, PermAppMembersManage); err != nil {
			return err
		}
		if err := c.Err(); err != nil {
			return err
		}
		oldOwners, oldPEs := userIDsOf(app.Owners), userIDsOf(app.PEs)
		if err := checkMembers(users, owners, oldOwners, pes, oldPEs); err != nil {
			return err
		}
		lists := []struct {
			role     MemberRole
			old, new []UserID
		}{{RoleOwner, oldOwners, owners}, {RolePE, oldPEs, pes}}
		var events []event
		for _, l := range lists { // removals first, then additions
			for _, id := range without(l.old, l.new) {
				events = append(events, memberRemovedEvent(l.role, id))
			}
		}
		for _, l := range lists {
			for _, id := range without(l.new, l.old) {
				events = append(events, memberAddedEvent(l.role, id))
			}
		}
		for _, l := range lists {
			if !slices.Equal(l.old, l.new) { // a pure reordering is stored but not recorded
				if err := tx.SetMembers(ctx, app.ID, l.role, l.new); err != nil {
					return err
				}
			}
		}
		for _, ev := range events {
			if err := s.recordEvent(ctx, tx, app.ID, actor, ev); err != nil {
				return err
			}
		}
		result, _, err = tx.GetApplicationByKey(ctx, key)
		return err
	})
	return result, err
}

// OwnedApplication is an application a user is an Owner of, with the number of
// OTHER enabled Owners it has (read under a lock, see the repository).
type OwnedApplication struct {
	Key                string
	Name               string
	OtherEnabledOwners int
}

// checkNotSoleOwner refuses to disable a user who is the only enabled Owner of
// some application. Applications where the other Owners are all disabled count
// too, so an application always keeps an Owner who can act. Call it inside the
// transaction that holds the lock on the user row.
func (s *Service) checkNotSoleOwner(ctx context.Context, tx Repository, id UserID) error {
	owned, err := tx.LockOwnedApplications(ctx, id)
	if err != nil {
		return err
	}
	var blocking []any
	for _, a := range owned {
		if a.OtherEnabledOwners == 0 {
			blocking = append(blocking, map[string]any{"key": a.Key, "name": a.Name})
		}
	}
	if len(blocking) == 0 {
		return nil
	}
	return Validation(Invalid("", CodeSoleOwner, map[string]any{"applications": blocking}))
}

func userIDsOf(refs []UserRef) []UserID {
	out := make([]UserID, len(refs))
	for i, r := range refs {
		out[i] = r.ID
	}
	return out
}

// without returns the IDs of list that are not in other, in order.
func without(list, other []UserID) []UserID {
	var out []UserID
	for _, id := range list {
		if !slices.Contains(other, id) {
			out = append(out, id)
		}
	}
	return out
}

// ListAssignableUsers returns the enabled users who can be chosen as Owner or PE,
// ordered by display name. Any logged-in user may read it (the Owner/PE pickers
// are used by Owners, who are not administrators); only names are exposed.
func (s *Service) ListAssignableUsers(ctx context.Context, _ Actor) ([]UserRef, error) {
	all, err := s.repo.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	var out []UserRef
	for _, u := range all {
		if u.User.Status == UserEnabled {
			out = append(out, refOf(u.User))
		}
	}
	slices.SortFunc(out, func(a, b UserRef) int {
		if c := strings.Compare(strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName)); c != 0 {
			return c
		}
		return strings.Compare(a.Username, b.Username)
	})
	return out, nil
}

// DeleteApplication deletes an application with its lines, versions and change
// history (the database cascades; nothing is deleted by hand). Administrators
// only, and only after typing the application key.
func (s *Service) DeleteApplication(ctx context.Context, actor Actor, key, confirmKey string) error {
	if err := requirePermission(actor, PermAppDelete); err != nil {
		return err
	}
	return s.repo.InTx(ctx, func(tx Repository) error {
		app, err := s.lockApplicationByKey(ctx, tx, key)
		if err != nil {
			return err
		}
		var c Collector
		if c.Required("confirmKey", confirmKey) && strings.TrimSpace(confirmKey) != app.Key {
			c.Add("confirmKey", CodeInvalid, nil)
		}
		if err := c.Err(); err != nil {
			return err
		}
		return tx.DeleteApplication(ctx, app.ID)
	})
}

const maxDeployInfoLength = 20000

// UpdateDeployInfo replaces the deployment info (free text). Owners and PEs may
// edit it; administrators may not (unless they are Owner or PE themselves).
func (s *Service) UpdateDeployInfo(ctx context.Context, actor Actor, key string, deployInfo *string) (Application, error) {
	var result Application
	err := s.repo.InTx(ctx, func(tx Repository) error {
		app, err := s.lockApplicationByKey(ctx, tx, key)
		if err != nil {
			return err
		}
		if err := requireAppPermission(actor, app, PermAppDeployEdit); err != nil {
			return err
		}
		if deployInfo == nil {
			return Validation(Invalid("deployInfo", CodeRequired, nil))
		}
		text := strings.TrimRight(*deployInfo, " \t\r\n") // free text: keep leading indentation
		var c Collector
		checkLength(&c, "deployInfo", text, maxDeployInfoLength)
		if err := c.Err(); err != nil {
			return err
		}
		if text != app.DeployInfo {
			if err := tx.SetDeployInfo(ctx, app.ID, text); err != nil {
				return err
			}
			if err := s.recordEvent(ctx, tx, app.ID, actor, deployInfoChangedEvent()); err != nil {
				return err
			}
		}
		result, _, err = tx.GetApplicationByKey(ctx, key)
		return err
	})
	return result, err
}

// lockApplicationByKey reads an application inside a transaction and locks its row.
func (s *Service) lockApplicationByKey(ctx context.Context, tx Repository, key string) (Application, error) {
	app, found, err := tx.GetApplicationByKey(ctx, key)
	if err != nil {
		return Application{}, err
	}
	if !found {
		return Application{}, newKind(KindNotFound, CodeNotFound)
	}
	if err := tx.LockApplication(ctx, app.ID); err != nil {
		return Application{}, err
	}
	// Re-read under the lock so the diff is against the committed state.
	app, _, err = tx.GetApplicationByKey(ctx, key)
	return app, err
}
