package domain

import (
	"errors"
	"strings"
)

// ErrorKind classifies a domain error. The HTTP layer maps kinds to status codes
// (see internal/httpapi/errors.go); the domain never deals in status codes.
type ErrorKind int

const (
	KindValidation      ErrorKind = iota + 1 // 422: a business or input rule rejected the request
	KindUnauthenticated                      // 401: no valid session, or login failed
	KindForbidden                            // 403: authenticated but not allowed
	KindNotFound                             // 404
	KindUnavailable                          // 503: a required external system (directory, code platform) cannot be reached
	KindTooManyRequests                      // 429: the caller is rate limited
)

// FieldError is one entry of the error envelope {"errors":[{field, code, params}]}.
//
//   - Field is the request field the error belongs to (JSON name, dotted for nesting,
//     e.g. "repo.path"); empty for errors about the request as a whole.
//   - Code is a stable lowerCamel identifier; the frontend renders it via the
//     i18n key "error.<code>". Codes are listed in codes.go.
//   - Params is an optional JSON object for message interpolation and conflict
//     details (e.g. {"line": {...}, "releasedAt": "..."}). Values may nest.
type FieldError struct {
	Field  string         `json:"field,omitempty"`
	Code   string         `json:"code"`
	Params map[string]any `json:"params,omitempty"`
}

// Error is the only error type the domain service returns for expected failures.
// Anything else is an internal error (500).
type Error struct {
	Kind   ErrorKind
	Errors []FieldError
}

func (e *Error) Error() string {
	codes := make([]string, len(e.Errors))
	for i, f := range e.Errors {
		codes[i] = f.Field + ":" + f.Code
	}
	return "domain error: " + strings.Join(codes, ",")
}

// AsError unwraps a domain *Error.
func AsError(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}

// Validation builds a 422 error from field errors.
func Validation(errs ...FieldError) *Error { return &Error{Kind: KindValidation, Errors: errs} }

// Invalid is shorthand for one field error.
func Invalid(field, code string, params map[string]any) FieldError {
	return FieldError{Field: field, Code: code, Params: params}
}

func newKind(kind ErrorKind, code string) *Error {
	return &Error{Kind: kind, Errors: []FieldError{{Code: code}}}
}

// Collector accumulates field errors so a request reports all invalid fields at once.
type Collector struct{ errs []FieldError }

func (c *Collector) Add(field, code string, params map[string]any) {
	c.errs = append(c.errs, FieldError{Field: field, Code: code, Params: params})
}

// Required adds CodeRequired when value is blank.
func (c *Collector) Required(field, value string) bool {
	if strings.TrimSpace(value) == "" {
		c.Add(field, CodeRequired, nil)
		return false
	}
	return true
}

// Err returns a validation error, or nil when nothing was collected.
func (c *Collector) Err() error {
	if len(c.errs) == 0 {
		return nil
	}
	return Validation(c.errs...)
}
