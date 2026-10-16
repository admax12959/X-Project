#!/bin/sh
# Mechanical repository checks that no unit test covers:
#   1. no merge conflict markers in tracked files
#   2. migration file names: <14-digit timestamp>_<name>.sql, timestamps unique
#   3. a migration that exists on main is never renamed or deleted
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

exit $fail
