SHELL := /bin/sh

.PHONY: help bootstrap fmt lint test race build check

help:
	@echo "Available targets:"
	@echo "  bootstrap  - verify repository structure"
	@echo "  fmt        - run formatting (when toolchains exist)"
	@echo "  lint       - run lint checks (when toolchains exist)"
	@echo "  test       - run tests (when toolchains exist)"
	@echo "  race       - run race checks for Go (if module exists)"
	@echo "  build      - build binaries/apps (when modules exist)"
	@echo "  check      - run lint + test"

bootstrap:
	@test -d agent
	@test -d collector
	@test -d correlator
	@test -d shared
	@test -d ui
	@test -d docs
	@test -d project_management
	@echo "Repository structure verified."

fmt:
	@if [ -f go.mod ]; then go fmt ./...; else echo "No go.mod yet, skipping Go fmt."; fi
	@if [ -f ui/package.json ]; then (cd ui && npm run format --if-present); else echo "No ui/package.json yet, skipping UI format."; fi

lint:
	@if [ -f go.mod ]; then golangci-lint run ./...; else echo "No go.mod yet, skipping Go lint."; fi
	@if [ -f ui/package.json ]; then (cd ui && npm run lint --if-present); else echo "No ui/package.json yet, skipping UI lint."; fi

test:
	@if [ -f go.mod ]; then go test ./...; else echo "No go.mod yet, skipping Go tests."; fi
	@if [ -f ui/package.json ]; then (cd ui && npm test --if-present); else echo "No ui/package.json yet, skipping UI tests."; fi

race:
	@if [ -f go.mod ]; then go test -race ./...; else echo "No go.mod yet, skipping race tests."; fi

build:
	@if [ -f go.mod ]; then go build ./...; else echo "No go.mod yet, skipping Go build."; fi
	@if [ -f ui/package.json ]; then (cd ui && npm run build --if-present); else echo "No ui/package.json yet, skipping UI build."; fi

check: lint test
