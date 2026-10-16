.PHONY: repo-check test test-ldap-contract test-storage-contract web-check web-build image check clean-test-containers

# Everything a ticket must pass before it is done.
check: repo-check test web-check web-build

# Conflict markers, migration file names (scripts/check-repo.sh).
repo-check:
	@scripts/check-repo.sh

# Go tests, including the HTTP API tests on a throwaway MySQL 8 container.
test:
	go vet ./...
	go test ./...

# Contract tests of the real LDAP adapter; need Docker, not part of `check`.
test-ldap-contract:
	go test -count=1 -tags ldapcontract ./internal/ldapdir

# Contract tests of the real object storage adapters: local disk (temporary directory) and the
# S3-compatible one (throwaway MinIO container); need Docker, not part of `check`.
test-storage-contract:
	go test -count=1 -tags objectstorecontract ./internal/objectstore

web-check:
	pnpm --dir web install --frozen-lockfile
	pnpm --dir web check

web-build:
	pnpm --dir web build

image:
	docker build -t x-project .

# Remove MySQL containers left behind by interrupted test runs.
clean-test-containers:
	docker ps -aq --filter label=xproj-test | xargs -r docker rm -f
