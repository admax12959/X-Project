// Typed API client. Pages call api.<resource>.<action>(); nothing else calls fetch.
import type {
  AdminUser,
  ApiFieldError,
  Application,
  ApplicationSummary,
  BindIdentityInput,
  ChangeEvent,
  CodePlatform,
  CodePlatformInput,
  CreateApplicationInput,
  CreateUserInput,
  IdentityKind,
  ForkInput,
  LineDetail,
  LineageNode,
  List,
  LoginInput,
  LoginMethod,
  Repo,
  RegisterVersionInput,
  RepoInput,
  SessionInfo,
  UpdateMeInput,
  UpdateUserInput,
  Version,
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
    /** 401 invalidCredentials, 422 field errors (e.g. usernameTakenByLocal) or 503 ldapUnreachable on failure. */
    login: (input: LoginInput) => request<SessionInfo>('POST', '/api/session', { body: input, expect401: true }),
    logout: () => request<void>('DELETE', '/api/session', { expect401: true }),
  },
  applications: {
    /** Any logged-in user. q matches key, name, repository path and Owner names. */
    list: (q = '') => request<List<ApplicationSummary>>('GET', `/api/applications${q.trim() ? `?q=${encodeURIComponent(q.trim())}` : ''}`),
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
  /** Administrator only (403 otherwise). There is no delete: users are disabled. */
  users: {
    list: () => request<List<AdminUser>>('GET', '/api/users'),
    create: (input: CreateUserInput) => request<AdminUser>('POST', '/api/users', { body: input }),
    update: (id: string, input: UpdateUserInput) => request<AdminUser>('PATCH', `/api/users/${id}`, { body: input }),
    disable: (id: string) => request<AdminUser>('POST', `/api/users/${id}/disable`),
    enable: (id: string) => request<AdminUser>('POST', `/api/users/${id}/enable`),
    /** 422 field errors (identityKindBound, identityTaken, ldapLoginNotFound, password rules); 503 ldapUnreachable. */
    bindIdentity: (id: string, input: BindIdentityInput) => request<AdminUser>('POST', `/api/users/${id}/identities`, { body: input }),
  },
  /** The logged-in user's own settings. */
  me: {
    get: () => request<AdminUser>('GET', '/api/me'),
    /** 422 ldapManaged on displayName / email / department for LDAP-bound users. */
    update: (input: UpdateMeInput) => request<AdminUser>('PATCH', '/api/me', { body: input }),
    /** 422 on password when the authentication fails (never 401), identityKindBound, identityTaken; 503 ldapUnreachable. */
    bindIdentity: (input: BindIdentityInput) => request<AdminUser>('POST', '/api/me/identities', { body: input }),
    /** 422 lastIdentity on the only identity. */
    unbindIdentity: (kind: IdentityKind) => request<AdminUser>('DELETE', `/api/me/identities/${kind}`),
  },
};
