package domain

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Maintenance lines and the lineage (spec #2, ticket #9). The mainline is created
// with the application (applications.go); maintenance lines are forked here.

// LineStatus is a maintenance line's lifecycle state. The mainline has none (empty).
// Stored and sent lowercase. The state machine (active <-> frozen -> retired) is the
// lifecycle ticket's (#10); forking only ever creates active lines.
type LineStatus string

const (
	LineActive  LineStatus = "active"
	LineFrozen  LineStatus = "frozen"
	LineRetired LineStatus = "retired"
)

// SourceVersion is the version a maintenance line was forked from: a fact fixed
// when the line is forked, never changed. It can sit on any line, including a retired one. Line is the
// line that holds the version (short form: ID, IsMain, Name and Branch only).
type SourceVersion struct {
	ID         int64
	Number     string
	ReleasedAt time.Time
	Line       Line
}

// NewLine is what the repository stores for a forked maintenance line.
type NewLine struct {
	ApplicationID   int64
	Name            string
	Branch          string
	SourceVersionID int64
	Reason          string
	PlannedEnd      *time.Time
	Note            string
	ForkedAt        time.Time
}

// ErrBranchTaken is returned by the Repository when the branch exists in the application.
var ErrBranchTaken = errors.New("branch already taken")

// LineRepository is the maintenance-line part of Repository.
type LineRepository interface {
	// GetVersion reads one version of the application (with its line, short form).
	GetVersion(ctx context.Context, applicationID, versionID int64) (Version, bool, error)
	// CreateLine inserts an active maintenance line. Returns ErrBranchTaken on a duplicate branch.
	CreateLine(ctx context.Context, l NewLine) (int64, error)
	// FindLineByBranch reads the line that holds the branch (the mainline's branch too), with a
	// locking read so it sees a row committed by a concurrent transaction (inside InTx).
	FindLineByBranch(ctx context.Context, applicationID int64, branch string) (Line, bool, error)
}

// ForkInput is the Owner's fork request. Strings are as sent by the client.
type ForkInput struct {
	SourceVersionID string
	Name            string
	Branch          string
	Reason          string
	PlannedEnd      string // optional, YYYY-MM-DD
	Note            string // optional
}

const (
	maxLineNameLength = 200 // app_lines.name
	maxLineTextLength = 2000
)

// ForkLine forks a maintenance line from any version of the application, including
// a version on a retired line. Owners only (administrators and PEs cannot). The new
// line is active and the server records the time of forking. Branch names are unique in
// the application, the mainline's included (branchTaken, with the conflicting line).
//
// Check order: application exists, actor is an Owner, fields, source version.
func (s *Service) ForkLine(ctx context.Context, actor Actor, appKey string, in ForkInput) (Line, error) {
	app, err := s.GetApplication(ctx, actor, appKey)
	if err != nil {
		return Line{}, err
	}
	if !actorOwns(app, actor) {
		return Line{}, newKind(KindForbidden, CodeForbidden)
	}
	var c Collector
	name, branch := strings.TrimSpace(in.Name), strings.TrimSpace(in.Branch)
	reason, note := strings.TrimSpace(in.Reason), strings.TrimSpace(in.Note)
	var sourceID int64
	if raw := strings.TrimSpace(in.SourceVersionID); c.Required("sourceVersionId", raw) {
		if id, perr := strconv.ParseInt(raw, 10, 64); perr != nil || id <= 0 {
			c.Add("sourceVersionId", CodeInvalid, nil)
		} else {
			sourceID = id
		}
	}
	if c.Required("name", name) {
		checkLength(&c, "name", name, maxLineNameLength)
	}
	if c.Required("branch", branch) && !validBranchName(branch) {
		c.Add("branch", CodeInvalid, nil)
	}
	if c.Required("reason", reason) {
		checkLength(&c, "reason", reason, maxLineTextLength)
	}
	checkLength(&c, "note", note, maxLineTextLength)
	var plannedEnd *time.Time
	if raw := strings.TrimSpace(in.PlannedEnd); raw != "" {
		if d, perr := time.Parse(dateLayout, raw); perr != nil {
			c.Add("plannedEnd", CodeInvalid, nil)
		} else {
			plannedEnd = &d
		}
	}
	if err := c.Err(); err != nil {
		return Line{}, err
	}

	var out Line
	err = s.repo.InTx(ctx, func(tx Repository) error {
		src, found, err := tx.GetVersion(ctx, app.ID, sourceID)
		if err != nil {
			return err
		}
		if !found {
			return Validation(Invalid("sourceVersionId", CodeInvalid, nil))
		}
		now := s.nowUTC()
		id, err := tx.CreateLine(ctx, NewLine{
			ApplicationID: app.ID, Name: name, Branch: branch, SourceVersionID: src.ID, Reason: reason,
			PlannedEnd: plannedEnd, Note: note, ForkedAt: now,
		})
		if errors.Is(err, ErrBranchTaken) {
			return s.branchTakenError(ctx, tx, app.ID, "branch", branch)
		}
		if err != nil {
			return err
		}
		out = Line{
			ID: id, Name: name, Branch: branch, CreatedAt: now, Status: LineActive,
			SourceVersion: &SourceVersion{ID: src.ID, Number: src.Number, ReleasedAt: src.ReleasedAt, Line: src.Line},
			Reason:        reason, PlannedEnd: plannedEnd, Note: note, ForkedAt: &now,
		}
		return s.recordEvent(ctx, tx, app.ID, actor, lineForkedEvent(out, src))
	})
	return out, err
}

// branchTakenError is branchTaken on the given field (branch, or mainBranch when the
// mainline is renamed) with the line that holds the branch.
func (s *Service) branchTakenError(ctx context.Context, tx Repository, applicationID int64, field, branch string) error {
	other, found, err := tx.FindLineByBranch(ctx, applicationID, branch)
	if err != nil {
		return err
	}
	var params map[string]any
	if found {
		params = map[string]any{"line": lineParam(other)}
	}
	return Validation(Invalid(field, CodeBranchTaken, params))
}

// LineageNode is one line of the lineage tree with what the tree table shows next
// to it. Children are the lines forked from versions of this line, ordered by the
// release date of the version they were forked from, then by creation.
type LineageNode struct {
	Line         Line
	VersionCount int
	Latest       *Version // by release date (never by number); nil when the line has no versions
	Children     []*LineageNode
}

// Lineage returns the application's lineage: the mainline as the root, maintenance
// lines nested under the line that holds their source version, to any depth. Any
// logged-in user may read it. A line forked from a retired line's version stays
// attached to that line.
func (s *Service) Lineage(ctx context.Context, actor Actor, appKey string) (*LineageNode, error) {
	app, err := s.GetApplication(ctx, actor, appKey)
	if err != nil {
		return nil, err
	}
	lines, err := s.repo.ListLines(ctx, app.ID)
	if err != nil {
		return nil, err
	}
	versions, err := s.repo.ListVersions(ctx, app.ID) // latest release first
	if err != nil {
		return nil, err
	}
	return buildLineage(lines, versions)
}

// buildLineage assembles the tree from the flat lists.
func buildLineage(lines []Line, versions []Version) (*LineageNode, error) {
	nodes := make(map[int64]*LineageNode, len(lines))
	for _, l := range lines {
		nodes[l.ID] = &LineageNode{Line: l}
	}
	for i := range versions { // latest first: the first one met is the line's latest
		if n := nodes[versions[i].LineID]; n != nil {
			n.VersionCount++
			if n.Latest == nil {
				n.Latest = &versions[i]
			}
		}
	}
	var root *LineageNode
	for _, l := range lines {
		n := nodes[l.ID]
		if l.IsMain {
			root = n
			continue
		}
		if l.SourceVersion == nil || nodes[l.SourceVersion.Line.ID] == nil {
			return nil, errors.New("lineage: a maintenance line's source version is on an unknown line")
		}
		parent := nodes[l.SourceVersion.Line.ID]
		parent.Children = append(parent.Children, n)
	}
	if root == nil {
		return nil, errors.New("lineage: application has no mainline")
	}
	for _, n := range nodes {
		slices.SortStableFunc(n.Children, func(a, b *LineageNode) int {
			if c := a.Line.SourceVersion.ReleasedAt.Compare(b.Line.SourceVersion.ReleasedAt); c != 0 {
				return c
			}
			return int(a.Line.ID - b.Line.ID)
		})
	}
	return root, nil
}
