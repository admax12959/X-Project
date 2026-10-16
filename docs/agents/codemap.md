# Code map

Where each kind of change goes. Read this instead of exploring; every ticket
in phase 2 re-read the same files to learn what is written here.

## A backend feature touches, in this order

| Step | Where |
|---|---|
| Domain rules and service methods | `internal/domain/<feature>.go` (service methods hang off `Service`) |
| Repository interface | declared next to the feature, then embedded in `Repository` in `internal/domain/model.go` |
| Error codes | `internal/domain/codes.go`; error kinds/status in `internal/domain/errors.go` |
| SQL | `internal/store/mysqlstore/<feature>.go` plus a new migration in `internal/store/mysqlstore/migrations/` (timestamp-named, embedded) |
| HTTP handlers and routes | `internal/httpapi/<feature>.go`; routes registered in `internal/httpapi/server.go` |
| Mail | outbox + bundled templates in `internal/domain/mailtemplates/{zh,en}` |
| Audit | event types in `internal/domain/audit.go`; rendering in `web/src/audit/render.ts` |
| Test doubles | `internal/fakes/` (clock, mailer, object store); wiring in `internal/apitest/env.go` |
| API tests | `test/api/<feature>_test.go`, against a MySQL 8 container; prior art: `password_recovery_test.go`, `totp_login_test.go` |

## A UI change touches

| Step | Where |
|---|---|
| Types and client | `web/src/api/types.ts`, `web/src/api/client.ts` |
| Pages and shared UI | `web/src/pages/`, `web/src/ui/`, department helpers in `web/src/org/` |
| Text | `web/src/i18n/zh/<x>.ts` and `web/src/i18n/en/<x>.ts`; keys must match (`pnpm check:i18n`); error text lives in `errors.ts` |
| Style | light theme only; reuse the existing console classes |
| Docs | a section in `docs/dev.md` |

## Merge hotspots

These files get a line from every ticket, so parallel branches conflict in
them. The fix is always to keep both sides:

`internal/domain/codes.go`, `internal/domain/model.go`,
`internal/domain/errors.go`, `internal/domain/service.go`,
`internal/apitest/env.go`, `web/src/api/types.ts`,
`web/src/i18n/*/errors.ts`, `web/src/i18n/*/login.ts`, `docs/dev.md`.

Check after merging: `error.rateLimited` and other i18n keys appear once
(a duplicate key fails `pnpm check`), and `gofmt -l internal/` is clean for
the files you touched.

## Verify

`make check` (go vet, go test with the MySQL container, web typecheck,
i18n parity, vitest, build). Redirect it to a log and read the tail only on
failure.
