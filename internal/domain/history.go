package domain

import (
	"context"
	"encoding/json"
	"strconv"
	"time"
)

// Change history (spec #2, 变更历史): an append-only log of key operations per
// application. An event is a TYPE plus structured PARAMS; the frontend renders it
// in the current UI language (web/src/history/render.ts and the history.event.*
// messages). User input (names, reasons, ...) goes into params verbatim and is
// never translated. docs/dev.md explains how to add an event type.
//
// Rules for params:
//   - JSON-serialisable values only (strings, numbers, bools, lists, maps).
//   - A reference to a user is userParam(id), never a name: users are never
//     deleted (ADR-0004) and the name is resolved when the history is read.
//   - The event is written in the same transaction as the change it records.

// EventType identifies what happened. The value is the wire format and the
// suffix of the frontend message key "history.event.<type>".
type EventType string

const (
	// EventAppCreated: application created. Params: owners = [userParam].
	EventAppCreated EventType = "appCreated"
	// EventVersionRegistered: version registered on a line. Params: number,
	// line = {id, isMain, name, branch} as the line was when it happened.
	EventVersionRegistered EventType = "versionRegistered"
)

// event is a change-history entry that has not been written yet. Build it with
// one constructor per type (see appCreatedEvent) so params stay consistent.
type event struct {
	typ    EventType
	params map[string]any
}

func appCreatedEvent(owners []UserID) event {
	refs := make([]any, len(owners))
	for i, id := range owners {
		refs[i] = userParam(id)
	}
	return event{EventAppCreated, map[string]any{"owners": refs}}
}

func versionRegisteredEvent(number string, line Line) event {
	return event{EventVersionRegistered, map[string]any{"number": number, "line": lineParam(line)}}
}

// userParam is the params form of a reference to a user.
func userParam(id UserID) map[string]any {
	return map[string]any{"userId": strconv.FormatInt(int64(id), 10)}
}

// StoredChangeEvent is a change-history row as the repository keeps it.
type StoredChangeEvent struct {
	ID            int64
	ApplicationID int64
	At            time.Time
	ActorID       UserID
	Type          EventType
	Params        []byte // JSON object
}

// HistoryRepository is the change-history part of Repository. There is deliberately
// no update and no delete: the history is append-only.
type HistoryRepository interface {
	AppendChangeEvent(ctx context.Context, e StoredChangeEvent) error
	// ListChangeEvents returns an application's events, newest first.
	ListChangeEvents(ctx context.Context, applicationID int64) ([]StoredChangeEvent, error)
}

// recordEvent appends ev to the application's history. Call it inside the same
// InTx as the change it describes, with the tx Repository.
func (s *Service) recordEvent(ctx context.Context, tx Repository, applicationID int64, actor Actor, ev event) error {
	params, err := json.Marshal(ev.params)
	if err != nil {
		return err
	}
	return tx.AppendChangeEvent(ctx, StoredChangeEvent{
		ApplicationID: applicationID, At: s.nowUTC(), ActorID: actor.User.ID, Type: ev.typ, Params: params,
	})
}

// ChangeEvent is a change-history entry ready for display. Params is the stored
// params with every user reference enriched: {"userId", "username", "displayName"}.
type ChangeEvent struct {
	ID     int64
	At     time.Time
	Actor  UserRef
	Type   EventType
	Params map[string]any
}

// ListChangeEvents returns the application's change history, newest first. Any
// logged-in user may read it.
func (s *Service) ListChangeEvents(ctx context.Context, _ Actor, appKey string) ([]ChangeEvent, error) {
	app, found, err := s.repo.GetApplicationByKey(ctx, appKey)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, newKind(KindNotFound, CodeNotFound)
	}
	stored, err := s.repo.ListChangeEvents(ctx, app.ID)
	if err != nil {
		return nil, err
	}
	users := userResolver{repo: s.repo, cache: map[UserID]UserRef{}}
	out := make([]ChangeEvent, 0, len(stored))
	for _, e := range stored {
		var params map[string]any
		if err := json.Unmarshal(e.Params, &params); err != nil {
			return nil, err
		}
		resolved, err := users.enrich(ctx, params)
		if err != nil {
			return nil, err
		}
		actor, err := users.ref(ctx, e.ActorID)
		if err != nil {
			return nil, err
		}
		out = append(out, ChangeEvent{ID: e.ID, At: e.At, Actor: actor, Type: e.Type, Params: resolved.(map[string]any)})
	}
	return out, nil
}

// userResolver turns user IDs into UserRefs, remembering what it has looked up.
type userResolver struct {
	repo  Repository
	cache map[UserID]UserRef
}

func (r userResolver) ref(ctx context.Context, id UserID) (UserRef, error) {
	if u, ok := r.cache[id]; ok {
		return u, nil
	}
	u, found, err := r.repo.GetUser(ctx, id)
	if err != nil {
		return UserRef{}, err
	}
	ref := UserRef{ID: id}
	if found {
		ref = refOf(u)
	}
	r.cache[id] = ref
	return ref, nil
}

// enrich walks decoded params and expands every {"userId": "..."} object.
func (r userResolver) enrich(ctx context.Context, v any) (any, error) {
	switch x := v.(type) {
	case map[string]any:
		if raw, ok := x["userId"].(string); ok && len(x) == 1 {
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return x, nil
			}
			ref, err := r.ref(ctx, UserID(id))
			if err != nil {
				return nil, err
			}
			return map[string]any{"userId": raw, "username": ref.Username, "displayName": ref.DisplayName}, nil
		}
		for k, e := range x {
			enriched, err := r.enrich(ctx, e)
			if err != nil {
				return nil, err
			}
			x[k] = enriched
		}
	case []any:
		for i, e := range x {
			enriched, err := r.enrich(ctx, e)
			if err != nil {
				return nil, err
			}
			x[i] = enriched
		}
	}
	return v, nil
}
