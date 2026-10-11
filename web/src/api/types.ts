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

/** Administrator view of a user (GET /api/users). */
export type AdminUser = {
  id: string;
  username: string;
  displayName: string;
  email: string;
  phone: string;
  department: string;
  status: 'enabled' | 'disabled';
  isAdmin: boolean;
  identities: { kind: 'local' | 'ldap'; loginName: string; boundAt: string; boundVia: string }[];
};

export type CreateUserInput = {
  username: string;
  displayName: string;
  email: string;
  phone: string;
  department: string;
  password: string;
};

/** Fields left out are unchanged. */
export type UpdateUserInput = Partial<Pick<CreateUserInput, 'username' | 'displayName' | 'email' | 'phone' | 'department'>>;

export type PlatformKind = 'gitlab' | 'github';

/** A registered code platform. The token is never returned; tokenTail is its last 4 characters ("" for short tokens). */
export type CodePlatform = {
  id: string;
  kind: PlatformKind;
  baseUrl: string;
  tokenTail: string;
  createdAt: string;
};

export type CodePlatformInput = { kind: PlatformKind | ''; baseUrl: string; token: string };

/** Every collection endpoint returns {items: [...]}. */
export type List<T> = { items: T[] };
