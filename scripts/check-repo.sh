#!/bin/sh
# Mechanical repository checks that no unit test covers:
#   1. no merge conflict markers in tracked files
#   2. migration file names: <14-digit timestamp>_<name>.sql, timestamps unique
#   3. a migration that exists on main is never renamed or deleted
#   4. credential changes go through changeCredential (CODING_STANDARDS.md): the locks, the
#      deletes of sessions and the security notice appear only where the allowlist says
# Exit 0 when clean; prints each offence and exits 1 otherwise.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
fail=0

markers=$(git grep -nIE '^(<<<<<<< |>>>>>>> |=======$)' -- . ':!*.md' ':!*.snap' ':!web/pnpm-lock.yaml' || true)
if [ -n "$markers" ]; then
  echo "conflict markers:"; echo "$markers"; fail=1
fi

dir=internal/store/mysqlstore/migrations
bad=$(ls "$dir" | grep -vE '^[0-9]{14}_[a-z0-9_]+\.sql$' || true)
if [ -n "$bad" ]; then
  echo "migration names must match <14 digits>_<name>.sql:"; echo "$bad"; fail=1
fi
dups=$(ls "$dir" | cut -c1-14 | sort | uniq -d || true)
if [ -n "$dups" ]; then
  echo "duplicate migration timestamps:"; echo "$dups"; fail=1
fi

if git rev-parse --verify -q origin/main >/dev/null; then
  gone=$(git diff --name-status --diff-filter=DR origin/main -- "$dir" || true)
  if [ -n "$gone" ]; then
    echo "migrations already on main must not be renamed or deleted:"; echo "$gone"; fail=1
  fi
fi

# 4. The calls that a credential change must not repeat by hand. Each allowlist entry is
#    <file>:<name>; credential_change.go owns all of them, the others only read under the lock
#    (the login verifies the identity it locked; SendInvitation reads whether a password is set).
allow='
internal/domain/credential_change.go:LockEmailIdentity
internal/domain/credential_change.go:DeleteUserSessions
internal/domain/credential_change.go:DeleteOtherUserSessions
internal/domain/credential_change.go:notifySecurity
internal/domain/service.go:LockEmailIdentity
internal/domain/auth_tokens.go:LockEmailIdentity
'
calls=$(git grep -nE '(\.(LockEmailIdentity|DeleteUserSessions|DeleteOtherUserSessions)\(|notifySecurity\()' -- 'internal/domain/*.go' ':!internal/domain/*_test.go' | grep -vE '^[^:]+:[0-9]+:[[:space:]]*//' || true)
stray=$(printf '%s\n' "$calls" | while IFS= read -r line; do
  [ -n "$line" ] || continue
  file=${line%%:*}
  for name in $(printf '%s\n' "$line" | grep -oE 'LockEmailIdentity|DeleteUserSessions|DeleteOtherUserSessions|notifySecurity' | sort -u); do
    printf '%s\n' "$allow" | grep -qxF "$file:$name" || echo "$line"
  done
done | sort -u || true)
if [ -n "$stray" ]; then
  echo "use changeCredential (internal/domain/credential_change.go) instead of calling these directly:"
  echo "$stray"; fail=1
fi

exit $fail
