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
	CodeEmailManaged   = "emailManaged"   // contact email of a user without LDAP: it is the login email, change that

	// login identities (ADR-0002)
	CodeLastIdentity      = "lastIdentity"      // unbinding the user's only login identity
	CodeIdentityKindBound = "identityKindBound" // the user already has an identity of this kind
	CodeIdentityTaken     = "identityTaken"     // another user already holds this login identity
	CodeLdapLoginNotFound = "ldapLoginNotFound" // administrator binding: the directory has no such login name
	CodeLoginEmailTaken   = "loginEmailTaken"   // another email identity already uses this login email
	CodeNoEmailIdentity   = "noEmailIdentity"   // the user has no email identity to change

	// administrator role
	CodeLastAdmin = "lastAdmin" // field "isAdmin"; the last enabled administrator cannot lose the role

	// passwords of email identities
	CodePasswordChangeRequired = "passwordChangeRequired" // HTTP 403; the user must change the administrator-set password first
	CodePasswordUnchanged      = "passwordUnchanged"      // the new password equals the current one

	// applications
	CodeKeyTaken        = "keyTaken"        // another application already has this key
	CodeUserUnavailable = "userUnavailable" // Owner/PE is not an existing enabled user; params {userId}

	CodeSoleOwner      = "soleOwner"      // disabling the only (enabled) Owner of applications; params {applications: [{key, name}]}
	CodeNoEnabledOwner = "noEnabledOwner" // field "owners"; the new Owner list would have no enabled user

	// versions
	CodeNumberTaken = "numberTaken" // field "number"; params {line: {id,isMain,name,branch}, releasedAt: "YYYY-MM-DD"} of the version that has it

	// lines (fork)
	CodeBranchTaken = "branchTaken" // field "branch"; params {line: {id,isMain,name,branch}} of the line that has it (the mainline's branch counts)

	// line lifecycle (#10)
	CodeInvalidTransition    = "invalidTransition"    // field "status"; params {from, to}; the state machine does not allow it
	CodeNoteRequiredOnFrozen = "noteRequiredOnFrozen" // field "note"; registering on a Frozen line needs a note
	CodeLineRetired          = "lineRetired"          // request-level; a Retired line takes no versions and no edits
	CodeLineIsMainline       = "lineIsMainline"       // request-level; the mainline has no status and no line edits

	// repository binding (field "repo.path" / "repo.platformId")
	CodeRepoNotFound        = "repoNotFound"        // the platform has no repository at this path
	CodeRepoForbidden       = "repoForbidden"       // the platform token cannot access this repository
	CodePlatformUnreachable = "platformUnreachable" // the code platform could not be reached; try again later

	// object storage (#24); the first four are the answers of "test connection"
	CodeStorageUnreachable    = "storageUnreachable"    // the storage could not be reached (network, DNS, TLS, timeout)
	CodeStorageAccessDenied   = "storageAccessDenied"   // the storage refused the access key or the operation
	CodeStorageBucketNotFound = "storageBucketNotFound" // the bucket (or local directory) does not exist
	CodeStorageFailed         = "storageFailed"         // anything else went wrong; the server log has the details
	CodeStorageUnavailable    = "storageUnavailable"    // HTTP 503; the configured storage cannot be used (the stored key is unreadable or the configuration is refused)

	// organization (#25)
	CodeDepartmentCycle    = "departmentCycle"    // field "parentId"; a department cannot move under itself or one of its descendants
	CodeDepartmentNotEmpty = "departmentNotEmpty" // request-level; params {members, children}; the department still has users or child departments

	// avatars (#32); the first two are field "avatar" errors
	CodeAvatarTooLarge        = "avatarTooLarge"        // more than 2 MB, or a picture with too many pixels; params {maxBytes, maxPixels}
	CodeAvatarUnsupportedType = "avatarUnsupportedType" // not a JPEG, PNG or WebP picture (SVG, a renamed file, a broken picture); params {allowed}
	CodeRateLimited           = "rateLimited"           // HTTP 429; too many requests of this kind from this user, try again later
)
