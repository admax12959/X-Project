package domain

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

// RefKind says what a version's reference points at (ADR-0001: never verified).
type RefKind string

const (
	RefTag    RefKind = "tag"
	RefCommit RefKind = "commit"
)

// Version is one manually registered release (GLOSSARY: 版本). It belongs to exactly
// one line, and its number is unique within the application. ReleasedAt is a
// calendar date (UTC midnight). Line is filled in by the repository reads.
type Version struct {
	ID            int64
	ApplicationID int64
	LineID        int64
	Line          Line
	Number        string
	RefKind       RefKind
	RefValue      string
	ReleasedAt    time.Time
	Note          string
	CreatedAt     time.Time
}

// NewVersion is what the repository stores for a registered version.
type NewVersion struct {
	ApplicationID int64
	LineID        int64
	Number        string
	RefKind       RefKind
	RefValue      string
	ReleasedAt    time.Time
	Note          string
	CreatedAt     time.Time
	CreatedBy     UserID
}

// ErrNumberTaken is returned by the Repository when the number already exists in the application.
var ErrNumberTaken = errors.New("version number already taken")

// VersionRepository is the version part of Repository.
type VersionRepository interface {
	// LockLine reads one of the application's lines and locks its row until the
	// transaction ends (use inside InTx): registering a version and changing the
	// line's status (later tickets) then cannot interleave. Reports false when the
	// line does not exist or belongs to another application.
	LockLine(ctx context.Context, applicationID, lineID int64) (Line, bool, error)
	// CreateVersion inserts a version. Returns ErrNumberTaken on a duplicate number.
	CreateVersion(ctx context.Context, v NewVersion) (int64, error)
	// FindVersionByNumberForShare reads a version by number with a locking read, so
	// it sees a row committed by a concurrent transaction (inside InTx).
	FindVersionByNumberForShare(ctx context.Context, applicationID int64, number string) (Version, bool, error)
	// ListVersions returns the application's versions, latest release date first,
	// then newest registration first (never ordered by number), with their line.
	ListVersions(ctx context.Context, applicationID int64) ([]Version, error)
}

// RegisterVersionInput is the Owner's request. Strings are as sent by the client.
type RegisterVersionInput struct {
	Number     string
	RefKind    string
	RefValue   string
	ReleasedAt string // YYYY-MM-DD; the client defaults it to today
	Note       string
}

const (
	maxVersionNumberLength = 100 // app_versions.number
	maxRefValueLength      = 255 // app_versions.ref_value
	maxVersionNoteLength   = 2000
	DateLayout             = "2006-01-02"
)

// parsedVersion is a RegisterVersionInput that passed the field checks.
type parsedVersion struct {
	number     string
	refKind    RefKind
	refValue   string
	releasedAt time.Time
	note       string
}

func parseVersionInput(in RegisterVersionInput) (parsedVersion, error) {
	var c Collector
	p := parsedVersion{
		number: strings.TrimSpace(in.Number), refValue: strings.TrimSpace(in.RefValue), note: strings.TrimSpace(in.Note),
	}
	if c.Required("number", p.number) {
		checkLength(&c, "number", p.number, maxVersionNumberLength)
	}
	switch kind := RefKind(strings.TrimSpace(in.RefKind)); {
	case kind == "":
		c.Add("ref.kind", CodeRequired, nil)
	case kind == RefTag || kind == RefCommit:
		p.refKind = kind
	default:
		c.Add("ref.kind", CodeInvalid, nil)
	}
	if c.Required("ref.value", p.refValue) {
		checkLength(&c, "ref.value", p.refValue, maxRefValueLength)
	}
	if c.Required("releasedAt", in.ReleasedAt) {
		d, err := time.Parse(DateLayout, strings.TrimSpace(in.ReleasedAt))
		if err != nil {
			c.Add("releasedAt", CodeInvalid, nil)
		}
		p.releasedAt = d
	}
	checkLength(&c, "note", p.note, maxVersionNoteLength)
	return p, c.Err()
}

// lineParam is the params form of a line inside an error or event: the conflict
// object that lets the UI say where something already is.
func lineParam(l Line) map[string]any {
	return map[string]any{"id": strconv.FormatInt(l.ID, 10), "isMain": l.IsMain, "name": l.Name, "branch": l.Branch}
}

// checkLineAcceptsVersion is where a line's state limits version registration:
//   - the mainline (no status) and Active lines accept versions without conditions;
//   - Frozen: a note is required (noteRequiredOnFrozen, field "note");
//   - Retired: nothing may be registered (lineRetired). The error is request-level
//     (no field): the line is in the URL, not in the body.
//
// It runs inside the transaction that holds the line's row lock, so a concurrent
// status change cannot slip in between the check and the insert.
func checkLineAcceptsVersion(c *Collector, line Line, note string) {
	switch line.Status {
	case LineFrozen:
		if note == "" {
			c.Add("note", CodeNoteRequiredOnFrozen, nil)
		}
	case LineRetired:
		c.Add("", CodeLineRetired, nil)
	}
}

// RegisterVersion registers a version on one line of the application. Owners only
// (administrators and PEs cannot). The line is a parameter, not an assumption:
// registerVersionOnLine applies the line's rules to whichever line it is given.
// The tag or commit is stored as typed and never checked against the repository
// (ADR-0001): no code platform call happens here.
//
// Check order: application exists, actor is an Owner, fields, line exists.
func (s *Service) RegisterVersion(ctx context.Context, actor Actor, appKey, lineID string, in RegisterVersionInput) (Version, error) {
	app, err := s.GetApplication(ctx, actor, appKey)
	if err != nil {
		return Version{}, err
	}
	if err := requireAppPermission(actor, app, PermVersionRegister); err != nil {
		return Version{}, err
	}
	p, err := parseVersionInput(in)
	if err != nil {
		return Version{}, err
	}
	id, err := parseLineID(lineID)
	if err != nil {
		return Version{}, err
	}
	var out Version
	err = s.repo.InTx(ctx, func(tx Repository) error {
		line, found, err := tx.LockLine(ctx, app.ID, id)
		if err != nil {
			return err
		}
		if !found {
			return newKind(KindNotFound, CodeNotFound)
		}
		out, err = s.registerVersionOnLine(ctx, tx, app, line, actor, p)
		return err
	})
	return out, err
}

// registerVersionOnLine does the work inside the transaction: line rules, insert,
// number conflict, change-history event.
func (s *Service) registerVersionOnLine(ctx context.Context, tx Repository, app Application, line Line, actor Actor, p parsedVersion) (Version, error) {
	var c Collector
	checkLineAcceptsVersion(&c, line, p.note)
	if err := c.Err(); err != nil {
		return Version{}, err
	}
	now := s.nowUTC()
	id, err := tx.CreateVersion(ctx, NewVersion{
		ApplicationID: app.ID, LineID: line.ID, Number: p.number, RefKind: p.refKind, RefValue: p.refValue,
		ReleasedAt: p.releasedAt, Note: p.note, CreatedAt: now, CreatedBy: actor.User.ID,
	})
	if errors.Is(err, ErrNumberTaken) {
		return Version{}, s.numberTakenError(ctx, tx, app.ID, p.number)
	}
	if err != nil {
		return Version{}, err
	}
	if err := s.recordEvent(ctx, tx, app.ID, actor, versionRegisteredEvent(p.number, line)); err != nil {
		return Version{}, err
	}
	return Version{
		ID: id, ApplicationID: app.ID, LineID: line.ID, Line: line, Number: p.number, RefKind: p.refKind,
		RefValue: p.refValue, ReleasedAt: p.releasedAt, Note: p.note, CreatedAt: now,
	}, nil
}

// numberTakenError is numberTaken with the conflicting version's line and release date.
func (s *Service) numberTakenError(ctx context.Context, tx Repository, applicationID int64, number string) error {
	other, found, err := tx.FindVersionByNumberForShare(ctx, applicationID, number)
	if err != nil {
		return err
	}
	var params map[string]any
	if found {
		params = map[string]any{"line": lineParam(other.Line), "releasedAt": other.ReleasedAt.Format(DateLayout)}
	}
	return Validation(Invalid("number", CodeNumberTaken, params))
}

// ListVersions returns all versions of an application on all its lines, latest
// release date first. Any logged-in user may read them.
func (s *Service) ListVersions(ctx context.Context, actor Actor, appKey string) ([]Version, error) {
	app, err := s.GetApplication(ctx, actor, appKey)
	if err != nil {
		return nil, err
	}
	return s.repo.ListVersions(ctx, app.ID)
}
