# How to build, run and test

Toolchain: Go 1.26, Node 24 + pnpm 12, Docker (with `docker-compose` or `docker compose`).

## Layout

```
cmd/xproject/              server entry point (config → app.Start → HTTP)
internal/app/              config loading (YAML file + XP_* env) and the single startup path
internal/domain/           domain service: ALL business rules, error codes, adapter ports
internal/store/mysqlstore/ MySQL repository (used only by the domain service) + migrations/
internal/httpapi/          HTTP layer: parse, authenticate, call the service, serialize
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
XP_DB_DSN='user:pw@tcp(127.0.0.1:3306)/xproject' XP_ADMIN_USERNAME=admin XP_ADMIN_PASSWORD=change-me-now go run ./cmd/xproject
pnpm --dir web install && pnpm --dir web dev   # http://localhost:5173, proxies /api to :8080 (XP_API_ORIGIN)
```

### Configuration

Optional YAML file named by `XP_CONFIG` (see `config.example.yaml`); environment variables override it.

| Env var | YAML | Default | Meaning |
|---|---|---|---|
| `XP_DB_DSN` | `database.dsn` | (required) | go-sql-driver DSN. `parseTime`, UTC and `time_zone='+00:00'` are forced. |
| `XP_DB_CONNECT_TIMEOUT` | `database.connect_timeout` | `90s` | Keep retrying the DB at startup this long. |
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

`app.Deps` passes them in. In production, nil means "not configured" (there is no real LDAP or GitLab/GitHub implementation yet).

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
