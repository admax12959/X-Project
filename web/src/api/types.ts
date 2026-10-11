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

/** PATCH /api/applications/{key} (Owners only). Fields left out are unchanged; the key and the repository cannot be edited. */
export type UpdateApplicationInput = Partial<Pick<CreateApplicationInput, 'name' | 'description' | 'mainBranch'>>;

/** PUT /api/applications/{key}/members (administrators and Owners): replaces both lists, user IDs. */
export type UpdateMembersInput = { owners: string[]; pes: string[] };

/** An enabled user as offered by the Owner/PE pickers (GET /api/assignable-users). */
export type AssignableUser = { id: string; username: string; displayName: string };

/** GET /api/applications/{key}/lines. The mainline has no status and no name of its own. */
export type Line = { id: string; isMain: boolean; name: string; branch: string };

export type RefKind = 'tag' | 'commit';

/** GET /api/applications/{key}/versions. ref is the tag or commit as typed (never verified); releasedAt is a date "YYYY-MM-DD". */
export type Version = {
  id: string;
  lineId: string;
  line: Line;
  number: string;
  ref: { kind: RefKind; value: string };
  releasedAt: string;
  note: string;
  createdAt: string;
};

export type RegisterVersionInput = { number: string; ref: { kind: RefKind; value: string }; releasedAt: string; note: string };

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
