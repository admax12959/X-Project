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
};
