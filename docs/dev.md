# How to build, run and test

Toolchain: Go 1.26, Node 24 + pnpm 12, Docker (with `docker-compose` or `docker compose`).

## Layout

```
cmd/xproject/              server entry point (config → app.Start → HTTP)
internal/app/              config loading (YAML file + XP_* env) and the single startup path
internal/domain/           domain service: ALL business rules, error codes, adapter ports
internal/store/mysqlstore/ MySQL repository (used only by the domain service) + migrations/
internal/httpapi/          HTTP layer: parse, authenticate, call the service, serialize
internal/codeplatform/     real GitLab / GitHub adapters (one API call each; tests use httptest)
internal/fakes/            test doubles for the directory (LDAP) and code platform adapters
internal/apitest/          API test harness (MySQL container, fresh DB per test, HTTP client)
test/api/                  the HTTP API tests (all in this one package)
web/                       React + Vite + TypeScript + Base UI + Tailwind SPA
```

Layering rules:

- `httpapi` never makes a business decision. It decodes the body, reads the session cookie, calls one `domain.Service` method, and maps the result to JSON.
- `domain` owns every rule, including permissions. It talks to storage only through `domain.Repository` and to the outside only through `domain.Directory` and `domain.CodePlatformAdapter`.
- `mysqlstore` is called only by `domain`.

## Run

One command builds the image (frontend + backend):

```sh
docker build -t x-project .
```

App + MySQL 8 locally (migrations run at startup; the initial admin is `admin` / `change-me-now`):

```sh
docker-compose up --build            # or: docker compose up --build
XP_PORT=18080 docker-compose -p mine up --build   # another host port / project name
```

Backend against your own MySQL, with the frontend dev server:

```sh
XP_DB_DSN='user:pw@tcp(127.0.0.1:3306)/xproject' XP_SECRET_KEY=dev-secret XP_ADMIN_USERNAME=admin XP_ADMIN_PASSWORD=change-me-now go run ./cmd/xproject
pnpm --dir web install && pnpm --dir web dev   # http://localhost:5173, proxies /api to :8080 (XP_API_ORIGIN)
```

### Configuration

Optional YAML file named by `XP_CONFIG` (see `config.example.yaml`); environment variables override it.

| Env var | YAML | Default | Meaning |
|---|---|---|---|
| `XP_DB_DSN` | `database.dsn` | (required) | go-sql-driver DSN. `parseTime`, UTC and `time_zone='+00:00'` are forced. |
| `XP_DB_CONNECT_TIMEOUT` | `database.connect_timeout` | `90s` | Keep retrying the DB at startup this long. |
| `XP_SECRET_KEY` | `secret_key` | (required) | Encrypts secrets stored in the database (code platform tokens, AES-256-GCM). Keep it stable: changing it makes stored tokens unreadable. |
| `XP_LISTEN` | `listen` | `:8080` | HTTP listen address. |
| `XP_WEB_DIR` | `web_dir` | empty (image: `/srv/web`) | Built SPA; empty serves the API only. |
| `XP_ADMIN_USERNAME` / `XP_ADMIN_PASSWORD` | `admin.username` / `admin.password` | — | Initial administrator. Required only on the very first start. |
| `XP_ADMIN_DISPLAY_NAME` / `XP_ADMIN_EMAIL` | `admin.display_name` / `admin.email` | username / empty | |
| `XP_SESSION_TTL` | `session.ttl` | `12h` | Session lifetime. |
| `XP_COOKIE_SECURE` | `session.cookie_secure` | `false` | Set `true` behind HTTPS. |

First start means the `system_state` row has never been marked as bootstrapped. Bootstrap then creates the local admin with a `bootstrap` login identity. Later starts ignore the admin settings: changing the password in config does nothing.

## Test

```sh
make check          # = go vet + go test ./... + pnpm check (typecheck + i18n) + pnpm build
go test ./...       # backend only; the API tests start their own MySQL container
pnpm --dir web check
```

How the API tests work (`internal/apitest`):

- `TestMain` in `test/api` calls `apitest.Main`. It starts one `mysql:8.4` container per test process with a unique name (`xproj-test-<random>`), a dynamic host port (`127.0.0.1::3306`), tmpfs storage and the label `xproj-test`. Parallel runs on one machine never collide.
- `apitest.New(t)` creates a fresh database for the test and starts the app on it through `app.Start`, the same path production uses (migrate, then bootstrap). It serves the app with `httptest`. Tests are independent and call `t.Parallel()`.
- `env.Client()` is a browser-like client with its own cookie jar. `env.AdminClient()` is already logged in as the initial admin.
- `env.Directory` and `env.CodePlatforms` are the fakes wired into the app. Configure them in the test, for example `env.Directory.AddAccount(...)` or `env.CodePlatforms.AddRepo(...)`.
- Test helpers in `test/api/helpers_test.go` and `application_create_test.go` (same package, reuse them): `registerPlatform(t, admin, "gitlab", glURL)` returns a platform ID; `env.CodePlatforms.AddRepo(glURL, "group/repo", "1182")` makes a repo resolvable; `createUser(t, admin, over)` makes a user through the API; `createApp(t, admin, platformID, ownerIDs, over)` creates an application through the API; `newAppFixture(t)` bundles env, admin, platform, a known repo `payments/payments-core` and an Owner candidate (`f.post(over)` posts a create request). Plain-user clients: `env.SeedUserClient(...)`, or log in as a user created with `createUser` (`LoginLocal(username, "initial-pw-1")`).
- `env.SeedUserClient(username, password)` returns a client logged in as a plain (non-administrator) user inserted directly into the database. Use it for permission tests until the user-management API exists. The test config sets `SecretKey` for you.
- `env.Restart(mutate)` simulates a process restart on the same database.
- To reuse an existing MySQL 8 server instead of a container, set `XP_TEST_MYSQL_DSN='root:pw@tcp(127.0.0.1:3306)/'`. The value has no database name and no parameters.
- Clean up containers left behind by a killed run with `make clean-test-containers`.

Keep **all** API tests in `test/api`. Every extra test package starts another container. Write tests as user stories through HTTP only: no SQL, no calls into internal functions.

## Conventions

### HTTP API

- Routes are registered in `internal/httpapi/server.go` `routes()`. Every `/api/*` route requires a session. Only routes registered with `publicRoute` skip that check. Unknown `/api` paths return 401 when logged out and 404 when logged in.
- JSON is camelCase. IDs are strings. Collections return `{"items": [...]}`.
- Session: HttpOnly cookie `xp_session` (SameSite=Lax). Only the token's SHA-256 is stored. `POST /api/session` logs in, `GET` returns `{user}`, `DELETE` logs out.

### Errors

Every non-2xx body has this shape:

```json
{"errors": [{"field": "username", "code": "required", "params": {}}]}
```

- `field` is the request field (JSON name, dotted for nesting such as `repo.path`). It is omitted for request-level errors.
- `code` is a stable lowerCamel code.
- `params` is optional. It is a JSON object that may nest (for example conflict details such as `{"line": {...}, "releasedAt": "..."}`).

| Domain kind | HTTP | Example codes |
|---|---|---|
| `KindValidation` | 422 | `required`, `invalid`, later `keyTaken`, `branchTaken`, `soleOwner`… |
| `KindUnauthenticated` | 401 | `unauthenticated`, `invalidCredentials` (failed login) |
| `KindForbidden` | 403 | `forbidden` |
| `KindNotFound` | 404 | `notFound` |
| malformed JSON | 400 | `badRequest` |
| anything else | 500 | `internal` (details only in the server log) |

The domain returns `*domain.Error`, built with `domain.Validation(...)`, a `domain.Collector` (to report all invalid fields at once), or `newKind`. Only `internal/httpapi/errors.go` maps kinds to status codes.

**Adding a domain error code**

1. Add a constant to `internal/domain/codes.go`.
2. Return it from the service: `domain.Validation(domain.Invalid("branch", domain.CodeBranchTaken, map[string]any{"line": ...}))`.
3. Add `'error.<code>'` to `web/src/i18n/en/errors.ts` and `web/src/i18n/zh/errors.ts`. Params become `{name}` placeholders.

`go test ./internal/domain` fails if a code has no English message. `pnpm check:i18n` fails if zh and en differ.

The frontend shows errors with `errorsByField(err)` (field → message, `''` = request-level). Keep the raw error in state and render the messages at render time, so they follow a language switch.

Codes added with applications (#6): `keyTaken` (field `key`), `userUnavailable` (fields `owners` / `pes`, params `{userId}`; the user does not exist or is disabled), `repoNotFound` and `repoForbidden` (field `repo.path`), `platformUnreachable` (field `repo.platformId`). All platform outcomes are plain 422s with their own code; there is no 5xx kind for them.

### Applications

- Routes: `GET/POST /api/applications` (`?q=` search), `GET /api/applications/{key}`, `GET /api/applications/{key}/lines`, `GET /api/applications/{key}/history`, `POST /api/repo-verifications`. Creating needs an administrator; everything else needs any session. IDs of users and platforms travel as strings; the domain parses them.
- A repository is always sent as `repo: {platformId, path}`, in `repo-verifications` as well as in create, so error fields are `repo.platformId` / `repo.path` everywhere. Create resolves the repo again on the server and stores the platform's project ID; a client-sent project ID is ignored. That is what makes "no creation without a passing verification" true.
- Storage: `applications` (key, name, description, `platform_id`, `repo_path`, `repo_project_id`, `deploy_info`), `application_members` (role `owner`/`pe`, ordered), `app_lines`, `change_events`. The table is `app_lines`, not `lines`, because LINES is a MySQL reserved word.
- The mainline branch name is stored once, on the mainline row of `app_lines` (`is_main`, unique per application). `Application.MainBranch` is read from it. Fork branches (#7) go into the same table, so `UNIQUE (application_id, branch)` already gives "branch taken, including the mainline branch". Changing the main branch (#8) means updating that row only. The mainline has an empty `name` and no status; the UI labels it with the glossary term.
- Immutability: the application key and the repository binding have no edit path. #8's update input must not contain `key` or `repo`; the domain rule, not the HTTP layer, drops them.
- Search (`ListApplications(query)`) matches key, name, repository path and Owner display name / username, case-insensitively, with `%` and `_` taken literally.
- `Service.ListLines` returns the mainline only for now; #7 extends `Line` and the endpoint.

### Change history

The append-only log of key operations (`change_events`). An event is a type plus structured params. The server never produces sentences; the frontend renders them in the current UI language. User input inside params (names, reasons, branch names...) is stored and displayed verbatim.

- The repository port only has `AppendChangeEvent` and `ListChangeEvents`: no update, no delete (applications deleting cascades in the DB, #9).
- Write events through `Service.recordEvent(ctx, tx, applicationID, actor, ev)` inside the same `InTx` as the change they describe. Events are built by one constructor per type in `internal/domain/history.go`.
- A reference to a user in params is `userParam(id)` (`{"userId": "12"}`), never a name. When the history is read, every such object is expanded to `{userId, username, displayName}` with the user's current name (users are never deleted, ADR-0004), so a rename shows up in old events. Any other value is stored as given.
- Events are listed newest first. The actor is its own column and is returned as `actor: {id, username, displayName}`.

**Adding an event type**

1. `internal/domain/history.go`: add a constant `EventFoo EventType = "foo"` (document its params next to it) and a constructor `fooEvent(...) event` that builds the params map. Use `userParam(id)` for users.
2. Call `s.recordEvent(ctx, tx, app.ID, actor, fooEvent(...))` in the same transaction as the change.
3. Frontend messages: add `'history.event.foo'` to `web/src/i18n/en/history.ts` and `zh/history.ts`. Placeholders are `{name}` and receive plain strings or numbers (`Params`).
4. Frontend renderer: add `foo: (params, locale) => ({ ... })` to `renderers` in `web/src/history/render.ts`. It turns params into the placeholder values (`userName(ref)` and `joinNames(names, locale)` help). Do not translate user input.
5. Test through the API: act, then `GET /api/applications/{key}/history` and assert type and params (see `application_search_history_test.go`).

`go test ./internal/domain` fails if an event type has no `history.event.<type>` message or no renderer entry.

### Migrations

Files live in `internal/store/mysqlstore/migrations/` and are embedded into the binary. They run at every startup, under a MySQL named lock.

**Adding a migration**

1. Create `YYYYMMDDHHMMSS_description.sql` with the current UTC time, for example `20261012093000_code_platforms.sql`. Timestamps keep parallel branches from colliding.
2. End each statement with `;` at the end of a line. Don't put `;` inside a statement. Use only whole-line `--` comments.
3. Never edit a migration that has been merged. Add a new one.

Applied files are recorded by name in `schema_migrations`. A file merged later with an older timestamp still runs. MySQL DDL is not transactional, so keep files small. Store timestamps as `DATETIME(6)` in UTC.

### Adapters

`internal/domain/adapters.go` defines the ports. Their fakes are in `internal/fakes`.

- `CodePlatformAdapter.ResolveRepo(ctx, PlatformConn, path)` returns `RepoFound` with a project ID, or `RepoNotFound`, `RepoForbidden` or `PlatformUnreachable`.
- `Directory.Authenticate(ctx, login, password)` and `Directory.Lookup(ctx, login)` return `DirectoryOK` with an entry, or `DirectoryBadPassword` (Authenticate only), `DirectoryNotFound`, `DirectoryDisabled` or `DirectoryUnreachable`. Never treat Unreachable as NotFound (ADR-0004).

`app.Deps` passes them in. `cmd/xproject` wires the real GitLab/GitHub adapter (`internal/codeplatform`, tokens come from the stored platform, decrypted by the domain with `tokens.open`). There is no real LDAP implementation yet: a nil `Directory` means "not configured".

Real adapter details worth knowing: GitLab gets `GET {base}/api/v4/projects/{path with %2F}` with `PRIVATE-TOKEN` (it answers 404, not 403, for projects the token cannot see); GitHub gets `GET /repos/{owner}/{repo}` on `api.github.com` (github.com) or `{base}/api/v3` (Enterprise), and a 403 with `X-RateLimit-Remaining: 0` counts as unreachable. Redirects are never followed. Contract tests against a real platform are not part of the default suite.

### i18n

All visible text goes through `t('key')` from `web/src/i18n`. Dictionaries are split by feature area: `web/src/i18n/{en,zh}/<area>.ts`.

**Adding an i18n key**

1. Add it to `en/<area>.ts` and `zh/<area>.ts`. Use the GLOSSARY term (English name for `en`), and never a word GLOSSARY lists under _Avoid_.
2. For a new area, create both files and register them in `en/index.ts` **and** `zh/index.ts` (`sections` and the merged object).
3. Run `pnpm --dir web check`. TypeScript rejects unknown keys in `t()`. `check:i18n` rejects keys or `{placeholders}` that differ between zh and en, empty messages, and duplicate keys across areas.

Prefixes: `term.*` (glossary), `common.*`, `error.*`, then one prefix per area (`login.*`, `apps.*`, …). User-entered content is never translated.

### Frontend

- All server calls go through `web/src/api/client.ts` (`api.<resource>.<action>`). Wire types are in `api/types.ts`.
- Any 401 (except the session calls themselves) drops the SPA to anonymous. `RequireSession` then redirects to `/login?next=…`.
- Styling uses Tailwind with the Console tokens in `src/index.css` (`text-text-2`, `border-line`, `bg-subtle`, `text-accent`…). Use Base UI controls from `src/ui/controls.tsx`.
- Add pages under `src/pages/`, routes in `src/main.tsx`, and nav entries in `src/shell/Shell.tsx`.
- Dependencies: the Docker build runs pnpm with a minimum release age, so a package version published only days ago can pass `pnpm install` locally and still fail `docker build`. Pin an older version in that case, and run `docker build` before calling a dependency change done.
