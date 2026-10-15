package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/admax12959/X-Project/internal/domain"
)

// Every non-2xx response has the body {"errors":[{"field"?, "code", "params"?}]}.
// Status mapping (the only place it lives):
//
//	KindValidation      → 422
//	KindUnauthenticated → 401 (includes failed login: code invalidCredentials)
//	KindForbidden       → 403
//	KindNotFound        → 404
//	KindUnavailable     → 503
//	malformed request   → 400 badRequest
//	anything else       → 500 internal (details only in the server log)
var kindStatus = map[domain.ErrorKind]int{
	domain.KindValidation:      http.StatusUnprocessableEntity,
	domain.KindUnauthenticated: http.StatusUnauthorized,
	domain.KindForbidden:       http.StatusForbidden,
	domain.KindNotFound:        http.StatusNotFound,
	domain.KindUnavailable:     http.StatusServiceUnavailable,
}

type errorEnvelope struct {
	Errors []domain.FieldError `json:"errors"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeCode(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, errorEnvelope{Errors: []domain.FieldError{{Code: code}}})
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	if de, ok := domain.AsError(err); ok {
		status, known := kindStatus[de.Kind]
		if known {
			writeJSON(w, status, errorEnvelope{Errors: de.Errors})
			return
		}
	}
	slog.Error("internal error", "method", r.Method, "path", r.URL.Path, "err", err)
	writeCode(w, http.StatusInternalServerError, domain.CodeInternal)
}

// decode reads a JSON body into v. It writes a 400 and returns false on failure.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(v); err != nil {
		writeCode(w, http.StatusBadRequest, domain.CodeBadRequest)
		return false
	}
	return true
}
