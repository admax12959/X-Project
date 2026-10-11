// Wire types. They mirror the JSON shapes in internal/httpapi/handlers.go.
// IDs are strings.

/** One entry of the error envelope {"errors": [...]} returned by every non-2xx response. */
export type ApiFieldError = {
  field?: string;
  code: string;
  params?: Record<string, unknown>;
};

export type User = {
  id: string;
  username: string;
  displayName: string;
  email: string;
  isAdmin: boolean;
};

export type SessionInfo = { user: User };

export type LoginInput = { method: 'local'; username: string; password: string };

export type ApplicationSummary = {
  key: string;
  name: string;
  description: string;
};

/** Every collection endpoint returns {items: [...]}. */
export type List<T> = { items: T[] };
