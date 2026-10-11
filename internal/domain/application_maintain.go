package domain

import (
	"context"
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
	// SetMainBranch renames the branch on the application's mainline row.
	SetMainBranch(ctx context.Context, applicationID int64, branch string) error
}

// isOwner reports whether the actor is one of the application's Owners.
func isOwner(actor Actor, a Application) bool { return containsUser(a.Owners, actor.User.ID) }

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
		if !isOwner(actor, app) {
			return forbidden()
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
			if err := tx.SetMainBranch(ctx, app.ID, branch); err != nil {
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
