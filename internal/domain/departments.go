package domain

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Organization (spec #18, #25; GLOSSARY: 组织架构, 部门). The company's departments form a
// tree kept in this tool. It displays and finds users and takes no part in authorization.
// A user belongs to at most one department (User.DepartmentID). The department synced from
// LDAP is something else: User.DirectoryDepartment, a read-only text.

// DepartmentID identifies a department.
type DepartmentID int64

// DepartmentSourceLocal marks departments maintained in this tool. Reserved for a later
// LDAP sync: such departments would carry another source and an external ID (read-only here).
const DepartmentSourceLocal = "local"

// Department is one node of the tree. A top-level department has no parent (ParentID 0).
type Department struct {
	ID          DepartmentID
	ParentID    DepartmentID // 0 = top level
	Name        string
	Source      string
	ExternalID  string // empty for local departments
	MemberCount int    // users directly in this department
	ChildCount  int    // departments directly under this one
	CreatedAt   time.Time
}

// ErrDepartmentMissing is returned by the Repository when a write refers to a department
// that does not exist (any more): the foreign key on the parent or on the user refuses it.
var ErrDepartmentMissing = errors.New("department does not exist")

// ErrDepartmentInUse is returned by the Repository when deleting a department that still
// has children or members: the foreign keys refuse it.
var ErrDepartmentInUse = errors.New("department has children or members")

// DepartmentRepository is the organization part of Repository.
type DepartmentRepository interface {
	// ListDepartments returns every department (oldest first) with its member and child counts.
	ListDepartments(ctx context.Context) ([]Department, error)
	GetDepartment(ctx context.Context, id DepartmentID) (Department, bool, error)
	// CreateDepartment returns ErrDepartmentMissing when the parent does not exist.
	CreateDepartment(ctx context.Context, d Department) (DepartmentID, error)
	// UpdateDepartment overwrites the name and the parent (0 = top level). ErrDepartmentMissing
	// when the parent does not exist.
	UpdateDepartment(ctx context.Context, id DepartmentID, name string, parent DepartmentID, at time.Time) error
	// DeleteDepartment returns ErrDepartmentInUse when it still has children or members.
	DeleteDepartment(ctx context.Context, id DepartmentID) error
	// LockDepartmentTree takes the lock that serializes restructuring (call inside InTx).
	LockDepartmentTree(ctx context.Context) error
	// DepartmentSubtree returns the department and all departments below it, at any depth.
	DepartmentSubtree(ctx context.Context, id DepartmentID) ([]DepartmentID, error)
}

// OptionalRef is a reference to a department in a request body that may be absent (Set
// false: leave unchanged), null or empty (Set with ID "": none / top level) or an ID.
type OptionalRef struct {
	Set bool
	ID  string
}

// parseDepartmentRef reads an ID from a request. Malformed IDs are reported on field.
func parseDepartmentRef(c *Collector, field, raw string) DepartmentID {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n <= 0 {
		c.Add(field, CodeInvalid, nil)
		return 0
	}
	return DepartmentID(n)
}

// ListDepartments returns the whole department tree as a flat list. Any logged-in user may
// read it (Owners filter the Owner/PE pickers by department).
func (s *Service) ListDepartments(ctx context.Context, _ Actor) ([]Department, error) {
	return s.repo.ListDepartments(ctx)
}

// DepartmentInput is a request to create a department. ParentID "" = top level.
type DepartmentInput struct {
	Name     string
	ParentID string
}

func checkDepartmentName(c *Collector, name string) {
	if name == "" {
		c.Add("name", CodeRequired, nil)
	} else {
		checkLength(c, "name", name, maxNameLength)
	}
}

// departmentRef is how a department appears in audit params: its ID and its name at that moment.
func departmentRef(d Department) map[string]any {
	return map[string]any{"id": strconv.FormatInt(int64(d.ID), 10), "name": d.Name}
}

// departmentRefOrNil is departmentRef for a department that may not exist (no parent, no department).
func departmentRefOrNil(d Department, ok bool) any {
	if !ok {
		return nil
	}
	return departmentRef(d)
}

func notFoundDepartment() error { return newKind(KindNotFound, CodeNotFound) }

// CreateDepartment adds a department under the parent (or at the top level).
func (s *Service) CreateDepartment(ctx context.Context, actor Actor, in DepartmentInput) (Department, error) {
	if err := requirePermission(actor, PermDepartmentManage); err != nil {
		return Department{}, err
	}
	var c Collector
	name := strings.TrimSpace(in.Name)
	checkDepartmentName(&c, name)
	parent := parseDepartmentRef(&c, "parentId", in.ParentID)
	if err := c.Err(); err != nil {
		return Department{}, err
	}
	var created Department
	err := s.repo.InTx(ctx, func(tx Repository) error {
		var parentDept Department
		if parent != 0 {
			var found bool
			var err error
			if parentDept, found, err = tx.GetDepartment(ctx, parent); err != nil {
				return err
			} else if !found {
				return Validation(Invalid("parentId", CodeInvalid, nil))
			}
		}
		now := s.nowUTC()
		d := Department{ParentID: parent, Name: name, Source: DepartmentSourceLocal, CreatedAt: now}
		id, err := tx.CreateDepartment(ctx, d)
		if errors.Is(err, ErrDepartmentMissing) {
			return Validation(Invalid("parentId", CodeInvalid, nil))
		}
		if err != nil {
			return err
		}
		d.ID = id
		created = d
		return s.recordAudit(ctx, tx, AuditActorOf(actor), departmentCreatedAudit(d, departmentRefOrNil(parentDept, parent != 0)))
	})
	return created, err
}

// UpdateDepartmentInput renames and/or moves a department. A nil Name and an unset Parent
// leave the field unchanged; Parent set with an empty ID moves it to the top level.
type UpdateDepartmentInput struct {
	Name   *string
	Parent OptionalRef
}

// UpdateDepartment renames a department and/or moves it (with everything below it) under
// another department or to the top level. A department cannot move under itself or one of
// its own descendants (422 departmentCycle on parentId).
func (s *Service) UpdateDepartment(ctx context.Context, actor Actor, id DepartmentID, in UpdateDepartmentInput) (Department, error) {
	if err := requirePermission(actor, PermDepartmentManage); err != nil {
		return Department{}, err
	}
	var c Collector
	var name string
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
		checkDepartmentName(&c, name)
	}
	var newParent DepartmentID
	if in.Parent.Set {
		newParent = parseDepartmentRef(&c, "parentId", in.Parent.ID)
	}
	if err := c.Err(); err != nil {
		return Department{}, err
	}
	var result Department
	err := s.repo.InTx(ctx, func(tx Repository) error {
		// Moving needs a tree nobody else is restructuring at the same time, or two moves
		// could each look fine and still close a cycle together.
		if in.Parent.Set {
			if err := tx.LockDepartmentTree(ctx); err != nil {
				return err
			}
		}
		cur, found, err := tx.GetDepartment(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return notFoundDepartment()
		}
		result = cur
		by := AuditActorOf(actor)

		finalName, finalParent := cur.Name, cur.ParentID
		if in.Name != nil {
			finalName = name
		}
		if in.Parent.Set {
			finalParent = newParent
		}
		var oldParentDept, newParentDept Department
		if in.Parent.Set && finalParent != cur.ParentID {
			if cur.ParentID != 0 {
				if oldParentDept, _, err = tx.GetDepartment(ctx, cur.ParentID); err != nil {
					return err
				}
			}
			if finalParent != 0 {
				var ok bool
				if newParentDept, ok, err = tx.GetDepartment(ctx, finalParent); err != nil {
					return err
				} else if !ok {
					return Validation(Invalid("parentId", CodeInvalid, nil))
				}
				below, err := tx.DepartmentSubtree(ctx, id)
				if err != nil {
					return err
				}
				if slices.Contains(below, finalParent) {
					return Validation(Invalid("parentId", CodeDepartmentCycle, nil))
				}
			}
		}
		if finalName == cur.Name && finalParent == cur.ParentID {
			return nil
		}
		if err := tx.UpdateDepartment(ctx, id, finalName, finalParent, s.nowUTC()); errors.Is(err, ErrDepartmentMissing) {
			return Validation(Invalid("parentId", CodeInvalid, nil))
		} else if err != nil {
			return err
		}
		result.Name, result.ParentID = finalName, finalParent
		if finalName != cur.Name {
			if err := s.recordAudit(ctx, tx, by, departmentRenamedAudit(cur, finalName)); err != nil {
				return err
			}
		}
		if finalParent != cur.ParentID {
			from := departmentRefOrNil(oldParentDept, cur.ParentID != 0)
			to := departmentRefOrNil(newParentDept, finalParent != 0)
			if err := s.recordAudit(ctx, tx, by, departmentMovedAudit(result, from, to)); err != nil {
				return err
			}
		}
		return nil
	})
	return result, err
}

// DeleteDepartment removes a department. A department that still has child departments or
// members cannot be deleted (422 departmentNotEmpty with the counts): move or reassign them first.
func (s *Service) DeleteDepartment(ctx context.Context, actor Actor, id DepartmentID) error {
	if err := requirePermission(actor, PermDepartmentManage); err != nil {
		return err
	}
	return s.repo.InTx(ctx, func(tx Repository) error {
		cur, found, err := tx.GetDepartment(ctx, id)
		if err != nil {
			return err
		}
		if !found {
			return notFoundDepartment()
		}
		inUse := func(d Department) error {
			return Validation(Invalid("", CodeDepartmentNotEmpty, map[string]any{"members": d.MemberCount, "children": d.ChildCount}))
		}
		if cur.MemberCount > 0 || cur.ChildCount > 0 {
			return inUse(cur)
		}
		if err := tx.DeleteDepartment(ctx, id); errors.Is(err, ErrDepartmentInUse) {
			// Somebody added a member or a child since we looked.
			again, _, gerr := tx.GetDepartment(ctx, id)
			if gerr != nil {
				return gerr
			}
			return inUse(again)
		} else if err != nil {
			return err
		}
		var parent Department
		if cur.ParentID != 0 {
			if parent, _, err = tx.GetDepartment(ctx, cur.ParentID); err != nil {
				return err
			}
		}
		return s.recordAudit(ctx, tx, AuditActorOf(actor), departmentDeletedAudit(cur, departmentRefOrNil(parent, cur.ParentID != 0)))
	})
}

// departmentFilter resolves the department filter of a user list: the department and all
// departments below it. An empty value means no filter (nil, false). A malformed or unknown
// ID is 422 invalid on departmentId.
func (s *Service) departmentFilter(ctx context.Context, raw string) (ids []DepartmentID, filtered bool, err error) {
	if strings.TrimSpace(raw) == "" {
		return nil, false, nil
	}
	var c Collector
	id := parseDepartmentRef(&c, "departmentId", raw)
	if err := c.Err(); err != nil {
		return nil, false, err
	}
	if _, found, err := s.repo.GetDepartment(ctx, id); err != nil {
		return nil, false, err
	} else if !found {
		return nil, false, Validation(Invalid("departmentId", CodeInvalid, nil))
	}
	ids, err = s.repo.DepartmentSubtree(ctx, id)
	return ids, true, err
}
