# Coding standards

Read during review, not implementation. Mechanical rules live in `scripts/check-repo.sh` and `make check`; this file holds the judgement calls.

## Credential changes and session issuance

Reviewing a change that sets a password, enrols or removes a second factor, binds or removes an email identity, or opens a session or login challenge:

- **Lock order and revoke together live in `changeCredential`** (`internal/domain/credential_change.go`), the one place that takes the user and email identity locks, the token, ends the challenges and sessions, and writes the audit entry and the notice. A new credential change goes through it and states its `RevokePolicy`; `scripts/check-repo.sh` fails on a direct `LockEmailIdentity`, `DeleteUserSessions`, `DeleteOtherUserSessions` or `notifySecurity` elsewhere. Review what the module cannot know: that the policy is the right one (`RevokeNone` is deliberate for the second-factor changes, which keep open sessions; the login-email change and the identity bind and unbind use it only because they have ended no session so far), and that a token's user is resolved first with the non-locking `FindAuthToken`.
- **Verify under the lock.** Reread the hash (or login name) after locking and compare it with the one that was verified. A login whose hash changed gets `invalidCredentials`.
- **Claim, then act.** A login challenge is spent by a delete that checks rows affected, and the session insert belongs to the same transaction. A one-time token is locked, revalidated under the lock, then marked used.
- **Notices are best-effort, but not blind.** A failed security-notice enqueue is logged and swallowed only when it is a statement-level failure; errors that abort the transaction (`IsTxAborting`) propagate.
- **Every password-only session path** rechecks, after inserting the session, whether a second factor became enabled; a session that already passed the second factor skips the recheck.

## Tests for security fixes

Each fix lands with a test that was red first. A race with no seam gets a fake-repository unit test that pins the order, plus a parallel API test that must never return a 5xx; say in the PR which of the two a case has.
