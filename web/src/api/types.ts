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
  /** Department text synced from LDAP; read-only, unrelated to the department tree. */
  directoryDepartment: string;
  /** Saved interface language; '' = never chosen. */
  language: Language | '';
  isAdmin: boolean;
};

export type Language = 'zh' | 'en';

/** mustChangePassword: the password was set by an administrator; until it is changed the server refuses everything else (403 passwordChangeRequired). */
export type SessionInfo = { user: User; mustChangePassword: boolean };

export type LoginMethod = 'email' | 'ldap';

/** email: the login email (not the username). ldap: the directory login name. */
export type LoginInput = { method: 'email'; email: string; password: string } | { method: 'ldap'; username: string; password: string };

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

/** The version released last on any line of an application (home page). */
export type LatestVersion = {
  id: string;
  number: string;
  /** Calendar date YYYY-MM-DD. */
  releasedAt: string;
  line: { id: string; isMain: boolean; name: string; branch: string };
};

/** One row of GET /api/applications (home page): the summary plus the latest version and the maintenance line count. */
export type ApplicationListItem = ApplicationSummary & {
  /** Maintenance lines in any status; the mainline is not counted. */
  lineCount: number;
  /** Null while no version is registered. */
  latestVersion: LatestVersion | null;
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

/** A line as referenced from other records (a version's line, a source version's line). The mainline has no name of its own. */
export type Line = { id: string; isMain: boolean; name: string; branch: string };

export type LineStatus = 'active' | 'frozen' | 'retired';

/** The version a maintenance line was forked from, with the line that holds it (that line's status: look it up in the lines list). */
export type SourceVersion = { id: string; number: string; releasedAt: string; line: Line };

/**
 * GET /api/applications/{key}/lines (flat, mainline first). The mainline has no status, sourceVersion,
 * reason, plannedEnd, note or forkedAt. plannedEnd is a date "YYYY-MM-DD"; forkedAt a timestamp.
 */
export type LineDetail = Line & {
  status?: LineStatus;
  sourceVersion?: SourceVersion;
  reason?: string;
  plannedEnd?: string;
  note?: string;
  forkedAt?: string;
};

/** One entry of a line's status log (oldest first): who moved it from one status to another, and when (a timestamp). */
export type LineStatusChange = { from: LineStatus; to: LineStatus; at: string; actor: { id: string; username: string; displayName: string } };

/** GET /api/applications/{key}/lines/{id}, and the response of PATCH / PUT status. statusChanges is [] for the mainline. */
export type LineWithLog = LineDetail & { statusChanges: LineStatusChange[] };

/**
 * PATCH /api/applications/{key}/lines/{id} (Owners). Absent = unchanged; "" clears note and plannedEnd. The Git branch,
 * source version and fork reason have no edit path.
 */
export type UpdateLineInput = { name?: string; note?: string; plannedEnd?: string };

/** GET /api/applications/{key}/lineage: the mainline as the root, children nested to any depth. */
export type LineageNode = LineDetail & {
  versionCount: number;
  latestVersion: { id: string; number: string; releasedAt: string } | null;
  children: LineageNode[];
};

export type ForkInput = { sourceVersionId: string; name: string; branch: string; reason: string; plannedEnd?: string; note?: string };

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

/** GET /api/versions: a version plus the application it belongs to. */
export type GlobalVersion = Version & { application: { key: string; name: string } };

/** One page of the global version list; total counts all matches, not just this page. */
export type VersionPage = List<GlobalVersion> & { total: number; page: number; pageSize: number };

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

/** One entry of GET /api/recent-activity: a change event plus the application it belongs to. */
export type RecentActivity = ChangeEvent & { application: { key: string; name: string } };

/** Administrator view of a user (GET /api/users). */
export type AdminUser = {
  id: string;
  username: string;
  displayName: string;
  email: string;
  phone: string;
  /** Department text synced from LDAP; read-only, unrelated to the department tree. */
  directoryDepartment: string;
  /** The department in the organization (the department tree); null = none. */
  departmentId: string | null;
  language: Language | '';
  status: 'enabled' | 'disabled';
  isAdmin: boolean;
  identities: LoginIdentity[];
};

export type IdentityKind = 'email' | 'ldap';

/** How an identity got attached to its user. */
export type BoundVia = 'firstLogin' | 'self' | 'admin' | 'bootstrap';

export type LoginIdentity = { kind: IdentityKind; loginName: string; boundAt: string; boundVia: BoundVia };

/**
 * Bind a login identity. ldap: loginName (+ password when binding your own: you
 * authenticate with it once). email: only an administrator binds it: loginName is the
 * login email and password a temporary one the user must change at the first login.
 */
export type BindIdentityInput = { kind: IdentityKind; loginName?: string; password?: string };

/**
 * What a user may change about themselves (GET/PATCH /api/me answers with the AdminUser shape).
 * The contact email is not here: it is the directory's for LDAP users and the login email otherwise.
 */
export type UpdateMeInput = Partial<Pick<AdminUser, 'displayName' | 'phone' | 'language'>>;

export type ChangePasswordInput = { currentPassword: string; newPassword: string };

/** password is a temporary one: the user must change it at the first login. */
export type CreateUserInput = {
  username: string;
  displayName: string;
  loginEmail: string;
  phone: string;
  password: string;
  /** Empty or left out = no department. */
  departmentId?: string;
};

/** Fields left out are unchanged. */
export type UpdateUserInput = Partial<Pick<CreateUserInput, 'username' | 'displayName' | 'loginEmail' | 'phone'>> & {
  /** Moves the user to a department; null takes the user out of any department. */
  departmentId?: string | null;
  /** Grants or revokes the administrator role; 422 lastAdmin when it would leave no enabled administrator. */
  isAdmin?: boolean;
};

/** Operator of an audit entry: a user, or the command line in the container (kind "cli", no user). */
export type AuditActor = { kind: 'user' | 'cli'; id?: string; username?: string; displayName?: string };

/** Object of an audit entry: a user (id = user ID, with the user's name) or an application (id = application key). */
export type AuditTarget = { type: 'user' | 'application' | 'department' | 'systemConfig'; id: string; username?: string; displayName?: string };

/**
 * GET /api/audit-log: an action type plus params, rendered in the UI language (src/audit/render.ts).
 * User references inside params are {userId, username, displayName}.
 */
export type AuditEntry = {
  id: string;
  at: string;
  action: string;
  actor: AuditActor;
  target: AuditTarget;
  params: Record<string, unknown>;
};

export type AuditPage = List<AuditEntry> & { total: number; page: number; pageSize: number };

/** actor: a user ID or "cli". from / to: RFC 3339 instants, both inclusive. */
export type AuditQuery = {
  actor?: string;
  targetType?: 'user' | 'application' | 'systemConfig';
  targetId?: string;
  action?: string;
  from?: string;
  to?: string;
  page?: number;
  pageSize?: number;
};

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

export type RoleScope = 'global' | 'application';

/** A built-in role (GET /api/roles, administrators only): permission keys like "line.fork"; descriptions live in the i18n dictionaries as perm.<key>. */
export type Role = { id: 'administrator' | 'owner' | 'pe'; scope: RoleScope; permissions: string[] };

/**
 * GET/PUT /api/system-config/ldap (administrators only). The service account password is write-only:
 * the response only says whether one is set. secretsUnreadable: a stored secret cannot be decrypted with
 * the current master key, enter it again. configured: saved at least once (false = defaults shown).
 */
export type LdapConfig = {
  enabled: boolean;
  url: string;
  startTls: boolean;
  skipVerify: boolean;
  bindDn: string;
  bindPasswordSet: boolean;
  baseDn: string;
  userFilter: string;
  loginAttr: string;
  nameAttr: string;
  emailAttr: string;
  departmentAttr: string;
  disabledFilter: string;
  timeoutSeconds: number;
  secretsUnreadable: boolean;
  configured: boolean;
  updatedAt?: string;
};

/** The whole LDAP form as sent by PUT and by "test connection". bindPassword empty/absent keeps the stored one. */
export type LdapConfigInput = Omit<LdapConfig, 'bindPasswordSet' | 'secretsUnreadable' | 'configured' | 'updatedAt'> & { bindPassword?: string };

/** Answer of every "test connection" route: ok, or ok=false with the error code (message "error.<code>"). A failed test is HTTP 200. */
export type ConnectionTest = { ok: boolean; code?: string };

/** Every collection endpoint returns {items: [...]}. */
export type List<T> = { items: T[] };

/** A node of the department tree, sent as a flat list (GET /api/departments): parentId is null for a top-level department. */
export type Department = {
  id: string;
  parentId: string | null;
  name: string;
  /** Where the department came from; "manual" today, reserved for a directory sync. */
  source: string;
  /** Direct members and direct child departments (a department with either cannot be deleted). */
  memberCount: number;
  childCount: number;
};
