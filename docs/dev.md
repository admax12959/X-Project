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
| `XP_LDAP_URL` / `ldap.url` | | empty | The one LDAP directory, `ldap://host:389` or `ldaps://host:636`. Empty = LDAP login is not configured (the login page offers local login only). Setting any other `XP_LDAP_*` value without a URL, or a URL without `base_dn`, stops startup. |
| `XP_LDAP_BASE_DN` | `ldap.base_dn` | (required with URL) | Users are searched in this subtree. |
| `XP_LDAP_BIND_DN` / `XP_LDAP_BIND_PASSWORD` | `ldap.bind_dn` / `ldap.bind_password` | anonymous | Service account for searching. Set both or neither. |
| `XP_LDAP_USER_FILTER` | `ldap.user_filter` | `(uid=%s)` | Finds one user; exactly one `%s` is replaced by the escaped login name. |
| `XP_LDAP_LOGIN_ATTR` / `XP_LDAP_NAME_ATTR` / `XP_LDAP_EMAIL_ATTR` / `XP_LDAP_DEPARTMENT_ATTR` | `ldap.login_attr` / `name_attr` / `email_attr` / `department_attr` | `uid` / `displayName` / `mail` / `department` | Attributes read from the directory. The login attribute is the canonical login name and becomes the username. |
| `XP_LDAP_DISABLED_FILTER` | `ldap.disabled_filter` | empty | Optional filter that matches a user entry when the account is disabled, for example Active Directory `(userAccountControl:1.2.840.113556.1.4.803:=2)`. Empty = never disabled. |
| `XP_LDAP_START_TLS` / `XP_LDAP_SKIP_VERIFY` / `XP_LDAP_TIMEOUT` | `ldap.start_tls` / `skip_verify` / `timeout` | `false` / `false` / `10s` | |
| `XP_SESSION_TTL` | `session.ttl` | `12h` | Session lifetime. |
| `XP_COOKIE_SECURE` | `session.cookie_secure` | `false` | Set `true` behind HTTPS. |

First start means the `system_state` row has never been marked as bootstrapped. Bootstrap then creates the local admin with a `bootstrap` login identity. Later starts ignore the admin settings: changing the password in config does nothing.

## Test

```sh
make check          # = go vet + go test ./... + pnpm check (typecheck + i18n + unit tests) + pnpm build
go test ./...       # backend only; the API tests start their own MySQL container
pnpm --dir web check
pnpm --dir web test   # frontend unit tests only (vitest, node environment)
```

Frontend unit tests are for pure logic extracted from components (no DOM, no React): `name.test.ts` next to the module, `import { describe, expect, it } from 'vitest'`. Component behaviour is checked in a browser.

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

LDAP adapter contract tests (not in the default set; need Docker, pull `osixia/openldap:1.5.0`):

```sh
make test-ldap-contract    # = go test -tags ldapcontract ./internal/ldapdir
```

They start a throwaway OpenLDAP container (unique name, dynamic port, label `xproj-test`). To use your own server set `XP_TEST_LDAP_URL`, `XP_TEST_LDAP_BASE_DN`, `XP_TEST_LDAP_ADMIN_DN` and `XP_TEST_LDAP_ADMIN_PASSWORD`; entries are created under `ou=xp-contract`. `go vet -tags ldapcontract ./internal/ldapdir` checks that they compile.

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
| `KindValidation` | 422 | `required`, `invalid`, `keyTaken`, `soleOwner`, later `branchTaken`… |
| `KindUnauthenticated` | 401 | `unauthenticated`, `invalidCredentials` (failed login) |
| `KindForbidden` | 403 | `forbidden` |
| `KindNotFound` | 404 | `notFound` |
| `KindUnavailable` | 503 | `ldapUnreachable`. A required external system cannot be reached; never report it as not found or as wrong credentials. Code platform tickets use it for an unreachable platform. |
| malformed JSON | 400 | `badRequest` |
| anything else | 500 | `internal` (details only in the server log) |

The domain returns `*domain.Error`, built with `domain.Validation(...)`, a `domain.Collector` (to report all invalid fields at once), or `newKind`. Only `internal/httpapi/errors.go` maps kinds to status codes.

**Adding a domain error code**

1. Add a constant to `internal/domain/codes.go`.
2. Return it from the service: `domain.Validation(domain.Invalid("branch", domain.CodeBranchTaken, map[string]any{"line": ...}))`.
3. Add `'error.<code>'` to `web/src/i18n/en/errors.ts` and `web/src/i18n/zh/errors.ts`. Params become `{name}` placeholders.

`go test ./internal/domain` fails if a code has no English message. `pnpm check:i18n` fails if zh and en differ.

The frontend shows errors with `errorsByField(err)` (field → message, `''` = request-level). Keep the raw error in state and render the messages at render time, so they follow a language switch.

Codes added with applications (#6): `keyTaken` (field `key`), `userUnavailable` (fields `owners` / `pes`, params `{userId}`; the user does not exist or is disabled), `repoNotFound` and `repoForbidden` (field `repo.path`), `platformUnreachable` (field `repo.platformId`). Not found and forbidden are 422s; an unreachable platform is `KindUnavailable` (503, like `ldapUnreachable`) and is never reported as "not found". Create calls the platform before opening its transaction.

### Applications

- Routes: `GET/POST /api/applications` (`?q=` search, `?mine=true` filter, see "Home page (#12)"), `GET /api/applications/{key}`, `GET /api/applications/{key}/lines`, `GET /api/applications/{key}/history`, `POST /api/repo-verifications`. Creating needs an administrator; everything else needs any session. IDs of users and platforms travel as strings; the domain parses them.
- A repository is always sent as `repo: {platformId, path}`, in `repo-verifications` as well as in create, so error fields are `repo.platformId` / `repo.path` everywhere. Create resolves the repo again on the server and stores the platform's project ID; a client-sent project ID is ignored. That is what makes "no creation without a passing verification" true.
- Storage: `applications` (key, name, description, `platform_id`, `repo_path`, `repo_project_id`, `deploy_info`), `application_members` (role `owner`/`pe`, ordered), `app_lines`, `change_events`. The table is `app_lines`, not `lines`, because LINES is a MySQL reserved word.
- The mainline branch name is stored once, on the mainline row of `app_lines` (`is_main`, unique per application). `Application.MainBranch` is read from it. Fork branches (#9) go into the same table, so `UNIQUE (application_id, branch)` already gives "branch taken, including the mainline branch". Changing the main branch (#8) means updating that row only. The mainline has an empty `name` and no status; the UI labels it with the glossary term.
- Immutability: the application key and the repository binding have no edit path. #8's update input must not contain `key` or `repo`; the domain rule, not the HTTP layer, drops them.
- Search (`ListApplications(query)`) matches key, name, repository path and Owner display name / username, case-insensitively, with `%` and `_` taken literally.
- `Service.ListLines` returns every line with its fork facts (see "Lines and lineage (#9)").

### Maintaining applications (#7)

Code: `internal/domain/application_maintain.go`. Permission matrix (checked in the domain, 403 `forbidden`; a plain user, a PE or an administrator who is not listed gets 403 on rows they may not use; roles add up, so an administrator who is also Owner may do the Owner rows):

| Route | Who | Body |
|---|---|---|
| `PATCH /api/applications/{key}` | Owner | `name`, `description`, `mainBranch` (all optional). `key` and `repo` are not in the input type: sending them changes nothing. |
| `PUT /api/applications/{key}/members` | administrator (ADR-0003) or Owner | `owners` and `pes`, both required (`pes` may be `[]`) |
| `PUT /api/applications/{key}/deploy-info` | Owner or PE | `deployInfo` (free text, required key, may be empty, max 20000 characters) |
| `DELETE /api/applications/{key}` | administrator | `{"confirmKey": "<key>"}`, 204. 403 is decided before 404, 404 before the 422 `confirmKey` (`required` / `invalid`). |
| `GET /api/assignable-users` | any session | `{items: [{id, username, displayName}]}`, enabled users only: the Owner/PE pickers (Owners cannot call `/api/users`) |

- Each edit runs in one `InTx` and locks the application row (`LockApplication`) before diffing, so concurrent edits do not interleave. An edit that changes nothing writes no event. A request that has any invalid field changes nothing.
- Members: the Owner list can never be empty (`owners` `required`). Only people who are **newly added** must be existing enabled users (`userUnavailable`, params `{userId}`); someone already on the list may stay even if disabled since, and can be removed but not re-added. Duplicates are dropped, a pure reordering is stored without an event.
- Events written (all with the real actor): `appInfoChanged` (`changes: {name|description|mainBranch: {from, to}}`), `deployInfoChanged` (no params), `memberAdded` / `memberRemoved` (`role: "owner"|"pe"`, `user: userParam`; one event per person, removals before additions). The user is its own nested value because `enrich` only expands objects that are exactly `{userId}`.
- Sole Owner (`DisableUser`): a user cannot be disabled while some application would be left without an **enabled** Owner other than them. 422 `soleOwner`, no field, params `{applications: [{key, name}]}`. So `[U, V]` with V already disabled still blocks U. The check runs only when the status actually changes (re-disabling is harmless) and takes locks in the order user row, then the applications (by key), then shared locks on the co-Owners' user rows (`LockOwnedApplications`); if two disables race, one fails with a deadlock rather than leaving an application without an Owner.
- Deleting is `DELETE FROM applications` and nothing else: lines, members and change events go through `ON DELETE CASCADE`. **A table that references an application or a line (versions, ...) must declare `ON DELETE CASCADE` on that foreign key** so delete keeps working.
- Changing `mainBranch` updates the mainline row of `app_lines` only. A branch that a maintenance line already has is `422 branchTaken` on field `mainBranch` (params `{line}`), and nothing of the edit is saved.
- Frontend: `ApplicationDialogs.tsx` holds the four dialogs; `useSetApplication()` (from `ApplicationPage`) replaces the loaded application after an edit. The disable error `soleOwner` is rendered by `UsersPage` as a list of links to the blocking applications.

### Versions (#8)

- Routes: `POST /api/applications/{key}/lines/{lineId}/versions` (register on one line) and `GET /api/applications/{key}/versions` (all lines, no pagination, no server-side filters: the Versions tab filters by line and tag/commit in the browser). Registering needs an Owner of that application: administrators and PEs get 403. Any session may read. Check order: application (404), Owner (403), fields (422), line (404, also for a line of another application).
- Body: `{number, ref: {kind: "tag"|"commit", value}, releasedAt: "YYYY-MM-DD", note}`. Errors: `number`, `ref.kind`, `ref.value`, `releasedAt` are `required` (kind also `invalid`, date `invalid`), `tooLong` on `number` (100), `ref.value` (255), `note` (2000). `releasedAt` has no server default; the dialog defaults to the browser's local date. The tag or commit is stored as typed and never checked (ADR-0001); registering makes no code platform call.
- A version JSON is `{id, lineId, line: {id,isMain,name,branch}, number, ref, releasedAt, note, createdAt}`. The list is ordered by `releasedAt` desc, then registration desc, never by number. "Latest version of a line" and "previous version" in the dialog mean the first list item of that line.
- `numberTaken` (field `number`) has params `{line: {id,isMain,name,branch}, releasedAt}` of the version that holds the number. Numbers are unique per application and compared exactly (`utf8mb4_bin`: `V1` and `v1` are different); the value is trimmed before the check. Table `app_versions`.
- **Line objects in params** (error params and event params) are always `{id, isMain, name, branch}` (`lineParam` in `internal/domain/versions.go`). The mainline has an empty `name`. Other tickets that report a conflicting or affected line (`branchTaken`, fork and status events) reuse it. On the frontend, `flatParams` in `api/errors.ts` turns any such object into its in-sentence label (`common.line.mainline` / `common.line.named`), so an error message can say `{line}`; other nested params are dropped.
- **Registering on other lines.** `Service.RegisterVersion` takes the line from the URL, locks its row (`LockLine`, `FOR UPDATE`, scoped to the application) inside the transaction and calls `registerVersionOnLine`, which applies the line rules in `checkLineAcceptsVersion` and then inserts. It is still empty: the lifecycle ticket (#10) fills it in (`LockLine` already returns `Line.Status`). Frozen requires a note (`noteRequiredOnFrozen`, field `note`), Retired refuses (`lineRetired`, request-level: no field, because the line is in the URL; the dialog shows it in its alert). Status changes must take the same line lock. The dialog (`RegisterVersionDialog`) already takes the line as a prop; the Versions tab passes the mainline, the Lineage panel passes the selected line.
- Event `versionRegistered` (params `number`, `line`): "Registered version X on the mainline" or "on “name”".

### Lines and lineage (#9)

Code: `internal/domain/lines.go`, `internal/store/mysqlstore/lines.go`, `internal/httpapi/lines.go`; frontend `pages/ApplicationLineage.tsx` (tree table), `LinePanel.tsx`, `ForkDialog.tsx`, `ui/LineStatusLabel.tsx`.

- Routes: `POST /api/applications/{key}/lines` (fork), `GET /api/applications/{key}/lines` (flat, mainline first, then by creation), `GET /api/applications/{key}/lineage` (tree). Fork needs an Owner of that application: administrators and PEs get 403. Any session may read. Check order: application (404), Owner (403), fields (422), source version (422).
- Fork body: `{sourceVersionId, name, branch, reason, plannedEnd?, note?}`. Errors: `sourceVersionId` `required`/`invalid` (also when the version belongs to another application), `name`/`branch`/`reason` `required`, `tooLong` (name 200, reason and note 2000), `branch` `invalid` (Git ref rules, `validBranchName`), `plannedEnd` `invalid` (date `YYYY-MM-DD`), and `branchTaken` (field `branch`, params `{line: {id,isMain,name,branch}}`, the mainline included). A fork can use a version on any line, retired lines included. The server sets `status: "active"` and `forkedAt`.
- **Line JSON** (`GET /lines`, fork response): `{id, isMain, name, branch}` plus, for maintenance lines only, `status`, `sourceVersion: {id, number, releasedAt, line: {id,isMain,name,branch}}`, `reason`, `forkedAt` (RFC 3339), and `plannedEnd` (date) and `note` when set. The mainline has none of these (they are omitted, never null). The short `{id,isMain,name,branch}` form stays for `version.line` and for params. To show the status of the line that holds a version, look the line up in `GET /lines`.
- **Status values are lowercase**: `active`, `frozen`, `retired` (`domain.LineActive`...; column `app_lines.status`, NULL on the mainline). The state machine is #10's; forking only creates `active`.
- **Lineage JSON** (`GET /lineage`): the mainline node itself, not wrapped in `items`. Node = line JSON + `versionCount`, `latestVersion` (`{id, number, releasedAt}` by release date, never by number; `null` without versions) and `children` (never null, nested to any depth). Children of a node are the lines whose source version is on that line, ordered by the release date of the source version, then creation order. Built by `buildLineage` from `ListLines` and `ListVersions`, so adding a field (graph column, #11) means extending `LineageNode` and `lineageNodeJSON`.
- Storage: columns `source_version_id`, `reason`, `status`, `planned_end`, `note`, `forked_at` on `app_lines`. `source_version_id` is indexed but has no foreign key on purpose: `app_versions` already references `app_lines`, and a key back would make the application delete (#7) depend on the order of two cascades. The domain checks that the source version belongs to the application (`GetVersion` is scoped to it). The application delete relies on the database cascades alone. `branch` uses the table's default collation (case- and accent-insensitive), so `Cust/X` conflicts with `cust/x`; `FindLineByBranch` uses the same column, so the reported conflict is always the row the unique key hit. `LockLine` and `ListLines` share `lineSelect` (lock with `FOR UPDATE OF l` so the source version rows stay unlocked).
- Event `lineForked` (params `line` = the new line, `number` = the source version's number, `sourceLine`): "Forked maintenance line “name” from version X". Registering on a maintenance line reuses `versionRegistered` with the line's name.
- Tests: `test/api/lineage_test.go` has `seedPaymentsCore(t)` (the prototype's payments-core: four levels, one version forked into two lines, a line without versions, irregular numbers, all three statuses) returning branch to line ID and number to version ID maps. Reuse it. **`env.SeedLineStatus(appKey, branch, status)` is test-only and bypasses the state machine** (no history event, no status time). Once #10 exists, test lifecycle behaviour through its API and keep the helper for fixtures only.
- Frontend: the selected line is in the URL (`/apps/:key/lineage?line=<id>`). The panel sits beside the tree inside the tab, not in the shell, and the tree drops the Source version column while a line is selected, so it fits at 1280px without scrolling sideways (the prototype's known issue). `LineStatusLabel` is the one place that renders a status (icon + text, no pills). Add columns in `ApplicationLineage` (`template` grid string and the row cells).
- **Graph column (#11).** Optional first column, off by default; the switch (`role=switch`) is remembered in `localStorage` key `xp.lineage.graph` (`1`/`0`; storage errors mean off). Layout is the pure `layoutGraph(rows)` in `pages/lineageGraph.ts` (tested by `lineageGraph.test.ts` with ASCII pictures; the payments-core shape is one of the cases); `LineageGraphCell.tsx` draws it, one small SVG per row. One dot per row in lane = depth; per row it returns line segments in lane (x) and row-height fraction (y) units. The rail of a parent runs down to its last visible child; **consecutive children with the same `sourceVersion.id` share one junction** (a half-lane sub-rail), so two lines forked from one version look different from two forks of different versions. A collapsed parent has no rail; the column width comes from `graphLanes(root)` (deepest lane of the whole tree), so collapsing does not move it. The graph is `aria-hidden` and carries meaning by shape, never colour: diamond mainline, filled circle Active, filled square Frozen, hollow circle Retired, dashed ring a line without versions (any status); the status itself stays in the Status column as icon + text. Row height is the constant `ROW_H` (35px, `LineageGraphCell.tsx`); rows set it as an inline style, so change it only there.

### Home page (#12)

Code: `internal/domain/activity.go`, `internal/store/mysqlstore/{home,activity}.go`, `internal/httpapi/activity.go`; frontend `pages/ApplicationsPage.tsx` (the home route `/`) and `pages/RecentActivity.tsx`.

- `GET /api/applications?q=&mine=` (any session). `mine=true` keeps the applications where the **current user is an Owner or a PE** (an `EXISTS` on `application_members`, so a user who holds both roles lists the application once); `""` and `false` mean no filter, anything else is `422 invalid` on `mine`. It combines with `q` (AND). An administrator who is not listed on an application does not "own" it. `ListApplications` takes an `ApplicationFilter` (`Query`, `MemberID`) in the repository and an `ApplicationListQuery` in the service.
- Each list item also has `lineCount` and `latestVersion`.
  - **`lineCount` = maintenance lines in any status** (Active, Frozen and Retired) **and not the mainline**. Decision: GLOSSARY defines a maintenance line as a fork off the mainline, and the prototype's list counts `!isMain`. Retired lines are still recorded lines, so they stay in the count. A mainline-only application has `0`.
  - **`latestVersion` = the version with the newest `releasedAt` on any line, mainline included**, ties broken by registration order (same rule as #8), never by number. Decision: the ticket says "发布时间最新的那个"; the prototype's column was "latest mainline version", which we did not follow. The value is `{id, number, releasedAt, line: {id,isMain,name,branch}}` (the short line form), `null` while no version exists, so the UI can say on which line it was released. Example: the LTS line's `v2026.08` (2026-08-12) beats its `2.3.2`, and a maintenance line's version beats an older mainline one.
  - Both are computed in one grouped query per page of the list (`GROUP BY` and `ROW_NUMBER()`), not per application.
- `GET /api/recent-activity?limit=` (any session): `{items}`, the newest change events of **all** applications (everyone sees every application), newest first (`occurred_at` desc, then ID desc). `limit` is 1-50, default 8, else `422 invalid` on `limit`. Every event type is included, so later event types (status changes, #10) show up without changes. An item is the history event JSON of `GET /applications/{key}/history` plus `application: {key, name}`; user references are expanded by the same code (`userResolver.changeEvent`).
- Frontend: the "My applications" filter is the `?mine=true` query of the page URL. The recent-activity table renders sentences with `renderEvent` from `history/render.ts`, exactly like the history tab: a new event type needs no change here. The home page shows nothing about planned end dates (spec #2, out of scope).

### Global versions (#13)

- `GET /api/versions?app=<key>&q=<text>&page=<n>&pageSize=<n>`: any session. Versions of all applications, ordered like the per-application list (`releasedAt` desc, then registration desc, never by number). Domain: `Service.ListAllVersions` in `internal/domain/versions_global.go`; store: `internal/store/mysqlstore/versions_global.go`.
- Response: `{items, total, page, pageSize}`. `total` counts every match, not just the page. Each item is the version JSON of #8 plus `application: {key, name}`. This is the paginated-list shape; the other collections are still plain `{items}`.
- `app` is an exact application key; an unknown key gives an empty page (not 404). `q` is a case-insensitive substring of the version number (trimmed, `%` and `_` literal); it is the only searched field. They combine with AND.
- `page` starts at 1 (default 1), `pageSize` is 1-100 (default 20). Anything else is `422 invalid` on `page` / `pageSize`. A page past the end is `200` with no items.
- Frontend: `/versions` (`pages/VersionsPage.tsx`), sidebar entry for every user. Filters and page are kept in the URL (`?app=&q=&page=`). Links to an application or a line go through `ui/links.ts` (`appPath`, `linePath`); `linePath` points at the Lineage tab with the line selected (`?line=<id>`, #9).

### Change history

The append-only log of key operations (`change_events`). An event is a type plus structured params. The server never produces sentences; the frontend renders them in the current UI language. User input inside params (names, reasons, branch names...) is stored and displayed verbatim.

- The repository port only has `AppendChangeEvent` and `ListChangeEvents`: no update, no delete (deleting an application cascades in the DB, #7).
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

`app.Deps` passes them in. In production, nil means "not configured". The real LDAP adapter is `internal/ldapdir`, built by `app.NewDirectory(cfg)` (nil when `ldap.url` is empty). `cmd/xproject` wires the real GitLab/GitHub adapter `internal/codeplatform` (tokens come from the stored platform, decrypted by the domain with `tokens.open`).

Real adapter details worth knowing: GitLab gets `GET {base}/api/v4/projects/{path with %2F}` with `PRIVATE-TOKEN` (it answers 404, not 403, for projects the token cannot see); GitHub gets `GET /repos/{owner}/{repo}` on `api.github.com` (github.com) or `{base}/api/v3` (Enterprise), and a 403 with `X-RateLimit-Remaining: 0` counts as unreachable. Redirects are never followed. Contract tests against a real platform are not part of the default suite.

### LDAP login (ADR-0002, ADR-0004)

- `POST /api/session` with `method: "ldap"`. `GET /api/login-methods` (public) lists `["local"]` or `["local","ldap"]`.
- The directory is asked on every login. Not found, disabled or wrong password all give `401 invalidCredentials`; unreachable gives `503 ldapUnreachable`. Nothing is deleted when the directory refuses: the user and the LDAP identity stay and work again when the account comes back.
- A known LDAP identity logs in to its user (whoever bound it) and overwrites display name, email and department. An unknown one creates a user (username = login name, binding source `firstLogin`) only if the username is free; otherwise `422 usernameTakenByLocal` on field `username` and nothing is stored. Never merged by email or username.
- `Service.CheckUsernameFreeInDirectory(ctx, field, username)` is the admin-side check, already called by `CreateLocalUser`: `422 usernameExistsInLdap` (a disabled directory account counts too) or `503 ldapUnreachable` when the directory cannot be queried; it never skips the check. With no directory configured it passes. Any other code path that creates a local user must call it too.
- Binding an LDAP identity to an existing user needs no change to login: a known LDAP identity signs in to whichever user holds it and the directory overwrites display name, email and department (see "Personal settings and login identities").

### Personal settings and login identities (ADR-0002)

Routes in `internal/httpapi/handlers_me.go`, rules in `internal/domain/me.go` and `identities.go`. The user view returned by all of them is the administrator shape (`id, username, displayName, email, phone, department, language, status, isAdmin, identities[{kind, loginName, boundAt, boundVia}]`).

| Route | Meaning |
|---|---|
| `GET /api/me` | Own profile with identities. |
| `PATCH /api/me` | Body fields are all optional: `displayName`, `email`, `department` (only for users without an LDAP identity, otherwise `422 ldapManaged` on a changed value; sending the current value is fine), `phone` (everyone), `language` (`zh` or `en`, else `422 invalid`). `username` and `isAdmin` are ignored. |
| `POST /api/me/identities` | Bind for yourself, `201`. `{"kind":"ldap","loginName":...,"password":...}`: the directory authenticates the pair first; any failure is `422 invalidCredentials` on `password` (never 401, which would log the SPA out), unreachable is `503 ldapUnreachable`. `{"kind":"local","password":...}`: the login name is the username, the password follows the create rules; there is no existing credential to authenticate with, the logged-in session is the proof. Binding source `self`. |
| `DELETE /api/me/identities/{kind}` | Unbind. `422 lastIdentity` (field `kind`) on the only identity, `404` when the user has none of that kind. |
| `POST /api/users/{id}/identities` | Administrator bind, same body but no LDAP password: the directory is only asked if the login name exists (`422 ldapLoginNotFound` on `loginName`, `503 ldapUnreachable`; a disabled directory account counts as existing and can be bound). Binding source `admin`. |

- Both bind routes: `422 identityKindBound` (field `kind`, one identity per kind) and `422 identityTaken` (field `loginName`, held by another user). The check runs under the user row lock; the unique-key race maps to `identityTaken` too.
- Binding an LDAP identity (self or admin) syncs display name, email and department from the directory at once when the directory returns the entry; every later LDAP login syncs again. The username stays and is locked from then on (ADR-0004).
- `language` is stored on the user (`users.language`, empty = never chosen) and is part of the `user` object of `POST/GET /api/session`. The SPA applies it after login and when it loads the session; the language switch in the top bar and the settings page save it with `PATCH /api/me`, the one on the login page stays local.

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
