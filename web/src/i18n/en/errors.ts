// One key per backend error code: "error.<code>" (internal/domain/codes.go).
// Params from the error envelope are available as {name} placeholders.
export default {
  'error.required': 'Required.',
  'error.invalid': 'Invalid value.',
  'error.badRequest': 'The request could not be understood.',
  'error.unauthenticated': 'Your session has ended. Sign in again.',
  'error.forbidden': 'You do not have permission to do this.',
  'error.notFound': 'Not found.',
  'error.internal': 'Something went wrong on the server. Try again later.',
  'error.network': 'Cannot reach the server.',
  'error.unknown': 'Unexpected error ({code}).',
  'error.invalidCredentials': 'Incorrect username or password.',
  'error.tooShort': 'Must be at least {min} characters.',
  'error.tooLong': 'Must be at most {max} characters.',
  'error.usernameTaken': 'This username is already taken.',
  'error.usernameLocked': 'The username of a user with an LDAP login identity cannot be changed.',
  'error.ldapManaged': 'This field is managed by LDAP and cannot be changed here.',
};
