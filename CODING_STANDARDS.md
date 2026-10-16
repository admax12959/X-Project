# Coding standards

Read during review, not implementation. Mechanical rules live in `scripts/check-repo.sh` and `make check`; this file holds the judgement calls.

## Credential changes and session issuance

Reviewing a change that sets a password, enrols or removes a second factor, binds or removes an email identity, or opens a session or login challenge:

- **Lock order.** One transaction takes the user row, then the email identity row (`LockEmailIdentity`), then the one-time token, then login challenges, then sessions. Resolve a token's user with the non-locking `FindAuthToken` first. A different order deadlocks against the command-line reset.
- **Verify under the lock.** Reread the hash (or login name) after locking and compare it with the one that was verified. A login whose hash changed gets `invalidCredentials`.
- **Claim, then act.** A login challenge is spent by a delete that checks rows affected, and the session insert belongs to the same transaction. A one-time token is locked, revalidated under the lock, then marked used.
- **Revoke together.** A password change or reset ends the user's other sessions and pending login challenges in the same transaction; only the session that made the change is upgraded. Exception: the administrator and command-line second-factor reset keeps open sessions.
- **Notices are best-effort, but not blind.** A failed security-notice enqueue is logged and swallowed only when it is a statement-level failure; errors that abort the transaction (`IsTxAborting`) propagate.
- **Every password-only session path** rechecks, after inserting the session, whether a second factor became enabled; a session that already passed the second factor skips the recheck.

## Tests for security fixes

Each fix lands with a test that was red first. A race with no seam gets a fake-repository unit test that pins the order, plus a parallel API test that must never return a 5xx; say in the PR which of the two a case has.
