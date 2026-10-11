package domain

// Error codes. Every code here needs an "error.<code>" key in both frontend
// dictionaries (web/src/i18n/*/errors.ts). Add new codes here, never inline strings.
const (
	// generic
	CodeRequired        = "required"        // field is blank
	CodeInvalid         = "invalid"         // field has a value outside the allowed set/format
	CodeBadRequest      = "badRequest"      // malformed JSON or wrong types (HTTP layer)
	CodeUnauthenticated = "unauthenticated" // no valid session
	CodeForbidden       = "forbidden"       // not allowed for this user
	CodeNotFound        = "notFound"        // resource or route does not exist
	CodeInternal        = "internal"        // unexpected server error

	// sessions
	CodeInvalidCredentials = "invalidCredentials" // wrong username or password (deliberately uniform)

	// users (field-level; params noted where present)
	CodeTooShort       = "tooShort"       // params {min}
	CodeTooLong        = "tooLong"        // params {max}
	CodeUsernameTaken  = "usernameTaken"  // another user already has this username
	CodeUsernameLocked = "usernameLocked" // LDAP-bound users keep their username (ADR-0004)
	CodeLdapManaged    = "ldapManaged"    // field is owned by the directory for LDAP-bound users
)
