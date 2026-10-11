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
  isAdmin: boolean;
};

export type SessionInfo = { user: User };

export type LoginMethod = 'local' | 'ldap';

export type LoginInput = { method: LoginMethod; username: string; password: string };

/** Owner / PE as shown in an application. */
export type Member = { id: string; username: string; displayName: string; status: 'enabled' | 'disabled' };

/** A verified repository binding. projectId is assigned by the code platform and is what the binding relies on; path is display only. */
export type Repo = { platformId: string; kind: PlatformKind; baseUrl: string; path: string; projectId: string };

export type RepoInput = { platformId: string; path: string };

/** One row of GET /api/applications?q= . */
export type ApplicationSummary = {
  key: string;
  name: string;
  description: string;
  repo: Repo;
  owners: Member[];
};

/** GET /api/applications/{key}. */
export type Application = ApplicationSummary & {
  mainBranch: string;
  pes: Member[];
  deployInfo: string;
  createdAt: string;
};

export type CreateApplicationInput = {
  key: string;
  name: string;
  description: string;
  repo: RepoInput;
  /** Empty = main. */
  mainBranch: string;
  /** User IDs. */
  owners: string[];
  pes: string[];
};

/** GET /api/applications/{key}/lines. The mainline has no status and no name of its own. */
export type Line = { id: string; isMain: boolean; name: string; branch: string };

/**
 * One change-history entry: a type plus structured params. The frontend renders the
 * sentence (src/history/render.ts). User references inside params are
 * {userId, username, displayName}.
 */
export type ChangeEvent = {
  id: string;
  at: string;
  actor: { id: string; username: string; displayName: string };
  type: string;
  params: Record<string, unknown>;
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
