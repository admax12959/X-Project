package domain

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Line lifecycle (spec #2, ticket #10): the status state machine and the Owner's
// edits of a maintenance line. The mainline has no status and no edits here.
//
//	active -> frozen
//	frozen -> active
//	frozen -> retired   (final)
//	active -> retired   not allowed: freeze first
//
// Every change is recorded twice: in the line's own status log (time and actor, shown
// in the line detail) and in the application's change history.

// StoredLineStatusChange is one row of a line's status log as the repository keeps it.
type StoredLineStatusChange struct {
	LineID  int64
	From    LineStatus
	To      LineStatus
	At      time.Time
	ActorID UserID
}

// LineStatusChange is a status log entry ready for display.
type LineStatusChange struct {
	From  LineStatus
	To    LineStatus
	At    time.Time
	Actor UserRef
}

// LineDetail is a line with its status log (oldest change first). The mainline has none.
type LineDetail struct {
	Line          Line
	StatusChanges []LineStatusChange
}

// LineLifecycleRepository is the lifecycle part of Repository.
type LineLifecycleRepository interface {
	// GetLine reads one line of the application (without locking).
	GetLine(ctx context.Context, applicationID, lineID int64) (Line, bool, error)
	// SetLineStatus overwrites the status of a maintenance line. Call it inside InTx,
	// after LockLine.
	SetLineStatus(ctx context.Context, lineID int64, status LineStatus) error
	AddLineStatusChange(ctx context.Context, c StoredLineStatusChange) error
	// ListLineStatusChanges returns a line's status log, oldest first.
	ListLineStatusChanges(ctx context.Context, lineID int64) ([]StoredLineStatusChange, error)
}

// lineTransitions is the state machine: from -> allowed targets.
// Retired is final; active cannot skip frozen.
var lineTransitions = map[LineStatus][]LineStatus{
	LineActive: {LineFrozen},
	LineFrozen: {LineActive, LineRetired},
}

func knownLineStatus(s LineStatus) bool {
	return s == LineActive || s == LineFrozen || s == LineRetired
}

func parseLineID(raw string) (int64, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || id <= 0 {
		return 0, newKind(KindNotFound, CodeNotFound)
	}
	return id, nil
}

// GetLine returns one line with its status log. Any logged-in user may read it.
func (s *Service) GetLine(ctx context.Context, actor Actor, appKey, lineID string) (LineDetail, error) {
	app, err := s.GetApplication(ctx, actor, appKey)
	if err != nil {
		return LineDetail{}, err
	}
	id, err := parseLineID(lineID)
	if err != nil {
		return LineDetail{}, err
	}
	line, found, err := s.repo.GetLine(ctx, app.ID, id)
	if err != nil {
		return LineDetail{}, err
	}
	if !found {
		return LineDetail{}, newKind(KindNotFound, CodeNotFound)
	}
	return s.lineDetail(ctx, s.repo, line)
}

func (s *Service) lineDetail(ctx context.Context, repo Repository, line Line) (LineDetail, error) {
	out := LineDetail{Line: line, StatusChanges: []LineStatusChange{}}
	if line.IsMain {
		return out, nil
	}
	stored, err := repo.ListLineStatusChanges(ctx, line.ID)
	if err != nil {
		return LineDetail{}, err
	}
	users := userResolver{repo: repo, cache: map[UserID]UserRef{}}
	for _, c := range stored {
		actor, err := users.ref(ctx, c.ActorID)
		if err != nil {
			return LineDetail{}, err
		}
		out.StatusChanges = append(out.StatusChanges, LineStatusChange{From: c.From, To: c.To, At: c.At, Actor: actor})
	}
	return out, nil
}

// SetLineStatus moves a maintenance line along the state machine. Owners only.
//
// Check order: application exists, actor is an Owner, status value, line exists,
// mainline, then the transition. The line row is locked (the same lock version
// registration takes), so a status change and a registration never interleave.
func (s *Service) SetLineStatus(ctx context.Context, actor Actor, appKey, lineID, status string) (LineDetail, error) {
	app, err := s.GetApplication(ctx, actor, appKey)
	if err != nil {
		return LineDetail{}, err
	}
	if !actorOwns(app, actor) {
		return LineDetail{}, forbidden()
	}
	to := LineStatus(strings.TrimSpace(status))
	var c Collector
	if c.Required("status", string(to)) && !knownLineStatus(to) {
		c.Add("status", CodeInvalid, nil)
	}
	if err := c.Err(); err != nil {
		return LineDetail{}, err
	}
	id, err := parseLineID(lineID)
	if err != nil {
		return LineDetail{}, err
	}
	var out LineDetail
	err = s.repo.InTx(ctx, func(tx Repository) error {
		line, found, err := tx.LockLine(ctx, app.ID, id)
		if err != nil {
			return err
		}
		if !found {
			return newKind(KindNotFound, CodeNotFound)
		}
		if line.IsMain {
			return Validation(Invalid("", CodeLineIsMainline, nil))
		}
		if !slices.Contains(lineTransitions[line.Status], to) {
			return Validation(Invalid("status", CodeInvalidTransition, map[string]any{"from": string(line.Status), "to": string(to)}))
		}
		if err := tx.SetLineStatus(ctx, line.ID, to); err != nil {
			return err
		}
		if err := tx.AddLineStatusChange(ctx, StoredLineStatusChange{
			LineID: line.ID, From: line.Status, To: to, At: s.nowUTC(), ActorID: actor.User.ID,
		}); err != nil {
			return err
		}
		line.Status = to
		out, err = s.lineDetail(ctx, tx, line)
		return err
	})
	return out, err
}
