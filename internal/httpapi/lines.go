package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
)

// sourceVersionJSON: the version a maintenance line was forked from, and the line that holds it.
type sourceVersionJSON struct {
	ID         string   `json:"id"`
	Number     string   `json:"number"`
	ReleasedAt string   `json:"releasedAt"`
	Line       lineJSON `json:"line"`
}

// lineDetailJSON is a line with its fork facts. Fields that do not apply are
// omitted: the mainline has no status, sourceVersion, reason, plannedEnd, note or
// forkedAt. status is "active", "frozen" or "retired". plannedEnd is a date.
type lineDetailJSON struct {
	ID            string             `json:"id"`
	IsMain        bool               `json:"isMain"`
	Name          string             `json:"name"`
	Branch        string             `json:"branch"`
	Status        string             `json:"status,omitempty"`
	SourceVersion *sourceVersionJSON `json:"sourceVersion,omitempty"`
	Reason        string             `json:"reason,omitempty"`
	PlannedEnd    string             `json:"plannedEnd,omitempty"`
	Note          string             `json:"note,omitempty"`
	ForkedAt      string             `json:"forkedAt,omitempty"`
}

func shortLineJSON(l domain.Line) lineJSON {
	return lineJSON{ID: strconv.FormatInt(l.ID, 10), IsMain: l.IsMain, Name: l.Name, Branch: l.Branch}
}

func toLineDetailJSON(l domain.Line) lineDetailJSON {
	out := lineDetailJSON{
		ID: strconv.FormatInt(l.ID, 10), IsMain: l.IsMain, Name: l.Name, Branch: l.Branch,
		Status: string(l.Status), Reason: l.Reason, Note: l.Note,
	}
	if sv := l.SourceVersion; sv != nil {
		out.SourceVersion = &sourceVersionJSON{
			ID: strconv.FormatInt(sv.ID, 10), Number: sv.Number, ReleasedAt: sv.ReleasedAt.Format("2006-01-02"), Line: shortLineJSON(sv.Line),
		}
	}
	if l.PlannedEnd != nil {
		out.PlannedEnd = l.PlannedEnd.Format("2006-01-02")
	}
	if l.ForkedAt != nil {
		out.ForkedAt = l.ForkedAt.UTC().Format(time.RFC3339Nano)
	}
	return out
}

// latestVersionJSON: the newest version of a line by release date.
type latestVersionJSON struct {
	ID         string `json:"id"`
	Number     string `json:"number"`
	ReleasedAt string `json:"releasedAt"`
}

// lineageNodeJSON is one line of the lineage tree: the line (with its fork facts)
// plus what the tree table shows. children is never null; latestVersion is null for
// a line without versions. Later tickets add fields here (for example the graph column).
type lineageNodeJSON struct {
	lineDetailJSON
	VersionCount  int                `json:"versionCount"`
	LatestVersion *latestVersionJSON `json:"latestVersion"`
	Children      []lineageNodeJSON  `json:"children"`
}

func toLineageNodeJSON(n *domain.LineageNode) lineageNodeJSON {
	out := lineageNodeJSON{lineDetailJSON: toLineDetailJSON(n.Line), VersionCount: n.VersionCount, Children: make([]lineageNodeJSON, 0, len(n.Children))}
	if v := n.Latest; v != nil {
		out.LatestVersion = &latestVersionJSON{ID: strconv.FormatInt(v.ID, 10), Number: v.Number, ReleasedAt: v.ReleasedAt.Format("2006-01-02")}
	}
	for _, c := range n.Children {
		out.Children = append(out.Children, toLineageNodeJSON(c))
	}
	return out
}

// lineage serves GET /api/applications/{key}/lineage: the root node (the mainline)
// with its children nested to any depth. Not a collection, so no "items".
func (s *server) lineage(w http.ResponseWriter, r *http.Request) {
	root, err := s.svc.Lineage(r.Context(), actorOf(r), r.PathValue("key"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toLineageNodeJSON(root))
}

// forkLine serves POST /api/applications/{key}/lines.
func (s *server) forkLine(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SourceVersionID string `json:"sourceVersionId"`
		Name            string `json:"name"`
		Branch          string `json:"branch"`
		Reason          string `json:"reason"`
		PlannedEnd      string `json:"plannedEnd"`
		Note            string `json:"note"`
	}
	if !decode(w, r, &body) {
		return
	}
	l, err := s.svc.ForkLine(r.Context(), actorOf(r), r.PathValue("key"), domain.ForkInput{
		SourceVersionID: body.SourceVersionID, Name: body.Name, Branch: body.Branch, Reason: body.Reason,
		PlannedEnd: body.PlannedEnd, Note: body.Note,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toLineDetailJSON(l))
}
