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

	// LDAP (ADR-0002, ADR-0004)
	CodeUsernameTakenByLocal = "usernameTakenByLocal" // first LDAP login, but a user already owns the username
	CodeUsernameExistsInLdap = "usernameExistsInLdap" // creating a local user whose username is a directory login name
	CodeLdapUnreachable      = "ldapUnreachable"      // directory could not be queried (HTTP 503); never "not found"
	// users (field-level; params noted where present)
	CodeTooShort       = "tooShort"       // params {min}
	CodeTooLong        = "tooLong"        // params {max}
	CodeUsernameTaken  = "usernameTaken"  // another user already has this username
	CodeUsernameLocked = "usernameLocked" // LDAP-bound users keep their username (ADR-0004)
	CodeLdapManaged    = "ldapManaged"    // field is owned by the directory for LDAP-bound users

	// login identities (ADR-0002)
	CodeLastIdentity      = "lastIdentity"      // unbinding the user's only login identity
	CodeIdentityKindBound = "identityKindBound" // the user already has an identity of this kind
	CodeIdentityTaken     = "identityTaken"     // another user already holds this login identity
	CodeLdapLoginNotFound = "ldapLoginNotFound" // administrator binding: the directory has no such login name

	// applications
	CodeKeyTaken        = "keyTaken"        // another application already has this key
	CodeUserUnavailable = "userUnavailable" // Owner/PE is not an existing enabled user; params {userId}

	// versions
	CodeNumberTaken = "numberTaken" // field "number"; params {line: {id,isMain,name,branch}, releasedAt: "YYYY-MM-DD"} of the version that has it

	// lines (fork)
	CodeBranchTaken = "branchTaken" // field "branch"; params {line: {id,isMain,name,branch}} of the line that has it (the mainline's branch counts)

	// repository binding (field "repo.path" / "repo.platformId")
	CodeRepoNotFound        = "repoNotFound"        // the platform has no repository at this path
	CodeRepoForbidden       = "repoForbidden"       // the platform token cannot access this repository
	CodePlatformUnreachable = "platformUnreachable" // the code platform could not be reached; try again later
)
