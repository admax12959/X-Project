package domain

import (
	"context"
	"slices"
	"strconv"
	"strings"
)

// Line lifecycle (spec #2, ticket #10): the status state machine and the Owner's
// edits of a maintenance line. The mainline has no status and no edits here.

// LineLifecycleRepository is the lifecycle part of Repository.
type LineLifecycleRepository interface {
	// SetLineStatus overwrites the status of a maintenance line. Call it inside InTx,
	// after LockLine.
	SetLineStatus(ctx context.Context, lineID int64, status LineStatus) error
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

// SetLineStatus moves a maintenance line along the state machine. Owners only.
//
// Check order: application exists, actor is an Owner, line exists, then the
// transition.
func (s *Service) SetLineStatus(ctx context.Context, actor Actor, appKey, lineID, status string) (Line, error) {
	app, err := s.GetApplication(ctx, actor, appKey)
	if err != nil {
		return Line{}, err
	}
	if !actorOwns(app, actor) {
		return Line{}, forbidden()
	}
	to := LineStatus(strings.TrimSpace(status))
	var c Collector
	if c.Required("status", string(to)) && !knownLineStatus(to) {
		c.Add("status", CodeInvalid, nil)
	}
	if err := c.Err(); err != nil {
		return Line{}, err
	}
	id, perr := strconv.ParseInt(strings.TrimSpace(lineID), 10, 64)
	if perr != nil || id <= 0 {
		return Line{}, newKind(KindNotFound, CodeNotFound)
	}
	var out Line
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
		line.Status = to
		out = line
		return nil
	})
	return out, err
}
