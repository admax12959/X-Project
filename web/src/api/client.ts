// Typed API client. Pages call api.<resource>.<action>(); nothing else calls fetch.
import type {
  AdminUser,
  ApiFieldError,
  Application,
  ApplicationListItem,
  AssignableUser,
  BindIdentityInput,
  ChangePasswordInput,
  ChangeEvent,
  CodePlatform,
  CodePlatformInput,
  CreateApplicationInput,
  CreateUserInput,
  IdentityKind,
  ForkInput,
  LineDetail,
  LineStatus,
  LineWithLog,
  LineageNode,
  UpdateLineInput,
  List,
  LoginInput,
  LoginMethod,
  Repo,
  Role,
  RecentActivity,
  RegisterVersionInput,
  RepoInput,
  SessionInfo,
  UpdateApplicationInput,
  UpdateMembersInput,
  UpdateMeInput,
  UpdateUserInput,
  Version,
  VersionPage,
} from './types.ts';

/** Thrown for every non-2xx response (and network failures, status 0, code "network"). */
export class ApiError extends Error {
  readonly status: number;
  readonly errors: ApiFieldError[];
  constructor(status: number, errors: ApiFieldError[]) {
    super(errors.map((e) => (e.field ? `${e.field}:${e.code}` : e.code)).join(', ') || `HTTP ${status}`);
    this.status = status;
    this.errors = errors;
  }
  /** Errors for one field ("" = errors about the whole request). */
  forField(field: string): ApiFieldError[] {
    return this.errors.filter((e) => (e.field ?? '') === field);
  }
}

let unauthenticatedHandler: () => void = () => {};
/** Called on any 401 except from requests made with {expect401: true}. The session layer sends the user to /login. */
export function onUnauthenticated(fn: () => void) {
  unauthenticatedHandler = fn;
}

type Options = { body?: unknown; expect401?: boolean };

async function request<T>(method: string, path: string, opts: Options = {}): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      credentials: 'same-origin',
      headers: opts.body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
    });
  } catch {
    throw new ApiError(0, [{ code: 'network' }]);
  }
  if (res.status === 204) return undefined as T;
  const data: unknown = await res.json().catch(() => undefined);
  if (res.ok) return data as T;
  const errors = (data as { errors?: ApiFieldError[] } | undefined)?.errors ?? [{ code: res.status >= 500 ? 'internal' : 'unknown' }];
  if (res.status === 401 && !opts.expect401) unauthenticatedHandler();
  throw new ApiError(res.status, errors);
}

export const api = {
  session: {
    /** Public: which login methods the login page may offer. */
    methods: () => request<List<LoginMethod>>('GET', '/api/login-methods', { expect401: true }),
    /** Current session; 401 (ApiError) when logged out. */
    get: () => request<SessionInfo>('GET', '/api/session', { expect401: true }),
    /** 401 invalidCredentials, 422 field errors (e.g. usernameTakenByLocal) or 503 ldapUnreachable on failure. A session with mustChangePassword can only change the password and log out. */
    login: (input: LoginInput) => request<SessionInfo>('POST', '/api/session', { body: input, expect401: true }),
    logout: () => request<void>('DELETE', '/api/session', { expect401: true }),
  },
  applications: {
    /**
     * Any logged-in user. q matches key, name, repository path and Owner names; mine keeps the applications where
     * the current user is an Owner or a PE (both must hold).
     */
    list: (q = '', mine = false) => {
      const qs = new URLSearchParams();
      if (q.trim()) qs.set('q', q.trim());
      if (mine) qs.set('mine', 'true');
      return request<List<ApplicationListItem>>('GET', `/api/applications${qs.size ? `?${qs}` : ''}`);
    },
    /** 404 notFound for an unknown key. */
    get: (key: string) => request<Application>('GET', `/api/applications/${encodeURIComponent(key)}`),
    /** Flat, mainline first, with the fork facts and status of every line. */
    lines: (key: string) => request<List<LineDetail>>('GET', `/api/applications/${encodeURIComponent(key)}/lines`),
    /** The mainline with its children nested to any depth. */
    lineage: (key: string) => request<LineageNode>('GET', `/api/applications/${encodeURIComponent(key)}/lineage`),
    /**
     * Owners only (403 otherwise). Forks an active maintenance line. 422 field errors: sourceVersionId, name, branch
     * (branchTaken with params {line}), reason, plannedEnd, note.
     */
    fork: (key: string, input: ForkInput) => request<LineDetail>('POST', `/api/applications/${encodeURIComponent(key)}/lines`, { body: input }),
    /** One line with its status log (time and actor of every status change, oldest first). 404 for an unknown line. */
    line: (key: string, lineId: string) =>
      request<LineWithLog>('GET', `/api/applications/${encodeURIComponent(key)}/lines/${encodeURIComponent(lineId)}`),
    /**
     * Owners only (403 otherwise). Name, note and planned end date of a maintenance line. 422: name, note, plannedEnd
     * field errors; request-level lineRetired (a retired line is read-only) and lineIsMainline.
     */
    updateLine: (key: string, lineId: string, input: UpdateLineInput) =>
      request<LineWithLog>('PATCH', `/api/applications/${encodeURIComponent(key)}/lines/${encodeURIComponent(lineId)}`, { body: input }),
    /**
     * Owners only (403 otherwise). active -> frozen, frozen -> active, frozen -> retired (final). 422 on field "status":
     * required, invalid, invalidTransition {from, to}; request-level lineIsMainline.
     */
    setLineStatus: (key: string, lineId: string, status: LineStatus) =>
      request<LineWithLog>('PUT', `/api/applications/${encodeURIComponent(key)}/lines/${encodeURIComponent(lineId)}/status`, { body: { status } }),
    /** Versions on all lines, latest release date first. */
    versions: (key: string) => request<List<Version>>('GET', `/api/applications/${encodeURIComponent(key)}/versions`),
    /**
     * Owners only (403 otherwise). 422 field errors: number (required, tooLong, numberTaken with params
     * {line, releasedAt}), ref.kind, ref.value, releasedAt, note.
     */
    registerVersion: (key: string, lineId: string, input: RegisterVersionInput) =>
      request<Version>('POST', `/api/applications/${encodeURIComponent(key)}/lines/${encodeURIComponent(lineId)}/versions`, { body: input }),
    /** Newest first. */
    history: (key: string) => request<List<ChangeEvent>>('GET', `/api/applications/${encodeURIComponent(key)}/history`),
    /** Administrators only. 422 field errors: key, name, mainBranch, owners, pes, repo.platformId, repo.path. */
    create: (input: CreateApplicationInput) => request<Application>('POST', '/api/applications', { body: input }),
    /** Owners only (403 otherwise). 422 field errors: name, description, mainBranch. */
    update: (key: string, input: UpdateApplicationInput) => request<Application>('PATCH', `/api/applications/${encodeURIComponent(key)}`, { body: input }),
    /** Administrators and Owners. 422: owners required (never empty), owners / pes userUnavailable {userId}. */
    setMembers: (key: string, input: UpdateMembersInput) => request<Application>('PUT', `/api/applications/${encodeURIComponent(key)}/members`, { body: input }),
    /** Owners and PEs. Free text; 422 deployInfo tooLong. */
    setDeployInfo: (key: string, deployInfo: string) =>
      request<Application>('PUT', `/api/applications/${encodeURIComponent(key)}/deploy-info`, { body: { deployInfo } }),
    /** Administrators only; confirmKey must equal the key (422 confirmKey). Lines, versions and history go with it. */
    remove: (key: string, confirmKey: string) => request<void>('DELETE', `/api/applications/${encodeURIComponent(key)}`, { body: { confirmKey } }),
  },
  /** Any logged-in user: enabled users for the Owner/PE pickers. */
  assignableUsers: {
    list: () => request<List<AssignableUser>>('GET', '/api/assignable-users'),
  },
  versions: {
    /**
     * Any logged-in user. All applications, latest release date first. app = application key (an unknown key
     * gives an empty page), q = part of the version number, page from 1, pageSize 1-100 (default 20).
     */
    list: (params: { app?: string; q?: string; page?: number; pageSize?: number } = {}) => {
      const qs = new URLSearchParams();
      if (params.app) qs.set('app', params.app);
      if (params.q?.trim()) qs.set('q', params.q.trim());
      if (params.page) qs.set('page', String(params.page));
      if (params.pageSize) qs.set('pageSize', String(params.pageSize));
      return request<VersionPage>('GET', `/api/versions${qs.size ? `?${qs}` : ''}`);
    },
  },
  activity: {
    /** Any logged-in user. Latest change events of all applications, newest first; limit 1-50 (default 8). */
    recent: (limit?: number) => request<List<RecentActivity>>('GET', `/api/recent-activity${limit ? `?limit=${limit}` : ''}`),
  },
  repos: {
    /** Administrators only. Errors: repo.path repoNotFound / repoForbidden, repo.platformId platformUnreachable. */
    verify: (repo: RepoInput) => request<{ repo: Repo }>('POST', '/api/repo-verifications', { body: { repo } }),
  },
  codePlatforms: {
    /** Administrators only (403 otherwise). */
    list: () => request<List<CodePlatform>>('GET', '/api/code-platforms'),
    /** 422 field errors on kind / baseUrl / token. */
    create: (input: CodePlatformInput) => request<CodePlatform>('POST', '/api/code-platforms', { body: input }),
  },
  /** Administrators only (403 otherwise). Read-only: built-in roles cannot be changed. */
  roles: {
    list: () => request<List<Role>>('GET', '/api/roles'),
  },
  /** Administrator only (403 otherwise). There is no delete: users are disabled. */
  users: {
    list: () => request<List<AdminUser>>('GET', '/api/users'),
    create: (input: CreateUserInput) => request<AdminUser>('POST', '/api/users', { body: input }),
    update: (id: string, input: UpdateUserInput) => request<AdminUser>('PATCH', `/api/users/${id}`, { body: input }),
    disable: (id: string) => request<AdminUser>('POST', `/api/users/${id}/disable`),
    enable: (id: string) => request<AdminUser>('POST', `/api/users/${id}/enable`),
    /** 422 field errors (identityKindBound, identityTaken, loginEmailTaken, ldapLoginNotFound, password rules); 503 ldapUnreachable. */
    bindIdentity: (id: string, input: BindIdentityInput) => request<AdminUser>('POST', `/api/users/${id}/identities`, { body: input }),
  },
  /** The logged-in user's own settings. */
  me: {
    get: () => request<AdminUser>('GET', '/api/me'),
    /** 422 ldapManaged on displayName / department for LDAP-bound users. */
    update: (input: UpdateMeInput) => request<AdminUser>('PATCH', '/api/me', { body: input }),
    /** 422 currentPassword invalidCredentials, newPassword tooShort / passwordUnchanged; 404 for a user without an email identity. */
    changePassword: (input: ChangePasswordInput) => request<void>('PUT', '/api/me/password', { body: input }),
    /** Only ldap: 422 on password when the authentication fails (never 401), identityKindBound, identityTaken; 503 ldapUnreachable. */
    bindIdentity: (input: BindIdentityInput) => request<AdminUser>('POST', '/api/me/identities', { body: input }),
    /** 422 lastIdentity on the only identity. */
    unbindIdentity: (kind: IdentityKind) => request<AdminUser>('DELETE', `/api/me/identities/${kind}`),
  },
};
