package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/admax12959/X-Project/internal/domain"
)

// versionJSON: ref is the tag or commit as typed (never verified); releasedAt is a
// calendar date "YYYY-MM-DD"; line is the line the version belongs to.
type versionJSON struct {
	ID     string   `json:"id"`
	LineID string   `json:"lineId"`
	Line   lineJSON `json:"line"`
	Number string   `json:"number"`
	Ref    struct {
		Kind  string `json:"kind"`
		Value string `json:"value"`
	} `json:"ref"`
	ReleasedAt string `json:"releasedAt"`
	Note       string `json:"note"`
	CreatedAt  string `json:"createdAt"`
}

func toVersionJSON(v domain.Version) versionJSON {
	out := versionJSON{
		ID: strconv.FormatInt(v.ID, 10), LineID: strconv.FormatInt(v.LineID, 10),
		Line:   lineJSON{ID: strconv.FormatInt(v.Line.ID, 10), IsMain: v.Line.IsMain, Name: v.Line.Name, Branch: v.Line.Branch},
		Number: v.Number, ReleasedAt: v.ReleasedAt.Format("2006-01-02"), Note: v.Note,
		CreatedAt: v.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
	out.Ref.Kind, out.Ref.Value = string(v.RefKind), v.RefValue
	return out
}

// registerVersion serves POST /api/applications/{key}/lines/{lineId}/versions.
func (s *server) registerVersion(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Number string `json:"number"`
		Ref    struct {
			Kind  string `json:"kind"`
			Value string `json:"value"`
		} `json:"ref"`
		ReleasedAt string `json:"releasedAt"`
		Note       string `json:"note"`
	}
	if !decode(w, r, &body) {
		return
	}
	v, err := s.svc.RegisterVersion(r.Context(), actorOf(r), r.PathValue("key"), r.PathValue("lineId"), domain.RegisterVersionInput{
		Number: body.Number, RefKind: body.Ref.Kind, RefValue: body.Ref.Value, ReleasedAt: body.ReleasedAt, Note: body.Note,
	})
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, toVersionJSON(v))
}

// listVersions serves GET /api/applications/{key}/versions: every version on every
// line, latest release date first.
func (s *server) listVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := s.svc.ListVersions(r.Context(), actorOf(r), r.PathValue("key"))
	if err != nil {
		writeError(w, r, err)
		return
	}
	out := listJSON[versionJSON]{Items: make([]versionJSON, 0, len(versions))}
	for _, v := range versions {
		out.Items = append(out.Items, toVersionJSON(v))
	}
	writeJSON(w, http.StatusOK, out)
}
