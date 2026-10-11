.PHONY: test web-check web-build image check clean-test-containers

# Everything a ticket must pass before it is done.
check: test web-check web-build

# Go tests, including the HTTP API tests on a throwaway MySQL 8 container.
test:
	go vet ./...
	go test ./...

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
