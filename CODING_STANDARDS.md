# Coding standards

Read during review, not implementation. Mechanical rules live in `scripts/check-repo.sh` and `make check`; this file holds the judgement calls.

## Credential changes and session issuance

Reviewing a change that sets a password, enrols or removes a second factor, binds or removes an email identity, or opens a session or login challenge:

- **Lock order.** One transaction takes the user row, then the email identity row (`LockEmailIdentity`), then the one-time token, then login challenges, then sessions. Resolve a token's user with the non-locking `FindAuthToken` first. A different order deadlocks against the command-line reset.
- **Verify under the lock.** Reread the hash (or login name) after locking and compare it with the one that was verified. A login whose hash changed gets `invalidCredentials`.
- **Claim, then act.** Spending a challenge or token is a delete that checks rows affected; the session insert belongs to the same transaction.
- **Revoke together.** A credential change ends the user's other sessions and pending login challenges in the same transaction. Only the session that made the change is upgraded.
- **Notices are best-effort, but not blind.** A failed security-notice enqueue is logged and swallowed only when it is a statement-level failure; errors that abort the transaction (`IsTxAborting`) propagate.
- **Every new session path** rechecks whether a second factor became enabled after the read.

## Tests for security fixes

Each fix lands with a test that was red first. A race with no seam gets a fake-repository unit test that pins the order, plus a parallel API test that must never return a 5xx; say in the PR which of the two a case has.
