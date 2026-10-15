package httpapi

import (
	"io"
	"net/http"
	"strconv"

	"github.com/admax12959/X-Project/internal/domain"
)

// avatarURL is where browsers fetch a user's avatar: through the app, with the content hash as
// the cache-busting version. nil (JSON null) = the user has no avatar.
func avatarURL(id domain.UserID, hash string) *string {
	if hash == "" {
		return nil
	}
	u := "/api/avatars/" + strconv.FormatInt(int64(id), 10) + "?v=" + hash
	return &u
}

func (s *server) avatarRoutes() {
	s.route("PUT /api/me/avatar", s.uploadAvatar)
	s.route("DELETE /api/me/avatar", s.removeAvatar)
	s.route("GET /api/avatars/{id}", s.getAvatar)
}

// uploadAvatar takes the picture as the raw request body. The size limit and everything else
// about the file is the domain's decision; this only reads one byte more than the limit so that
// a file that is too big is noticed without reading all of it.
func (s *server) uploadAvatar(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(io.LimitReader(r.Body, domain.MaxAvatarBytes+1))
	if err != nil {
		writeCode(w, http.StatusBadRequest, domain.CodeBadRequest)
		return
	}
	u, err := s.svc.SetAvatar(r.Context(), actorOf(r), data)
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toAdminUserJSON(u))
}

func (s *server) removeAvatar(w http.ResponseWriter, r *http.Request) {
	u, err := s.svc.RemoveAvatar(r.Context(), actorOf(r))
	if err != nil {
		writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, toAdminUserJSON(u))
}

// getAvatar serves the user's current avatar to any logged-in user (the session check is the
// default of every route). The object is addressed by user, never by a client-supplied key; the
// "v" query parameter only busts caches, so it is not looked at. The picture is the same for as
// long as its hash is, hence the one-year cache, private because only logged-in users may have it.
func (s *server) getAvatar(w http.ResponseWriter, r *http.Request) {
	id, ok := userIDParam(w, r)
	if !ok {
		return
	}
	hash, err := s.svc.AvatarHash(r.Context(), id)
	if err != nil {
		writeError(w, r, err)
		return
	}
	h := w.Header()
	h.Set("Cache-Control", "private, max-age=31536000, immutable")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("ETag", `"`+hash+`"`)
	if r.Header.Get("If-None-Match") == `"`+hash+`"` {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	jpg, _, err := s.svc.ReadAvatar(r.Context(), id)
	if err != nil {
		h.Del("Cache-Control")
		h.Del("ETag")
		writeError(w, r, err)
		return
	}
	h.Set("Content-Type", "image/jpeg")
	h.Set("Content-Length", strconv.Itoa(len(jpg)))
	_, _ = w.Write(jpg)
}
