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
  department: string;
  /** Saved interface language; '' = never chosen. */
  language: Language | '';
  isAdmin: boolean;
};

export type Language = 'zh' | 'en';

export type SessionInfo = { user: User };

export type LoginMethod = 'local' | 'ldap';

export type LoginInput = { method: LoginMethod; username: string; password: string };

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
  language: Language | '';
  status: 'enabled' | 'disabled';
  isAdmin: boolean;
  identities: LoginIdentity[];
};

export type IdentityKind = 'local' | 'ldap';

/** How an identity got attached to its user. */
export type BoundVia = 'firstLogin' | 'self' | 'admin' | 'bootstrap';

export type LoginIdentity = { kind: IdentityKind; loginName: string; boundAt: string; boundVia: BoundVia };

/**
 * Bind a login identity. ldap: loginName (+ password when binding your own: you
 * authenticate with it once). local: password only; the login name is the username.
 */
export type BindIdentityInput = { kind: IdentityKind; loginName?: string; password?: string };

/** What a user may change about themselves (GET/PATCH /api/me answers with the AdminUser shape). */
export type UpdateMeInput = Partial<Pick<AdminUser, 'displayName' | 'email' | 'phone' | 'department' | 'language'>>;

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
