SHELL := /bin/sh

.PHONY: help bootstrap fmt lint test race build check release-artifacts

help:
	@echo "Available targets:"
	@echo "  bootstrap  - verify repository structure"
	@echo "  fmt        - run formatting (when toolchains exist)"
	@echo "  lint       - run lint checks (when toolchains exist)"
	@echo "  test       - run tests (when toolchains exist)"
	@echo "  race       - run race checks for Go (if module exists)"
	@echo "  build      - build binaries/apps (when modules exist)"
	@echo "  check      - run lint + test"
	@echo "  release-artifacts VERSION=<semver> - create source archive + checksum"

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
	@if [ -f go.mod ]; then \
		if command -v go >/dev/null 2>&1; then \
			go fmt ./...; \
		else \
			echo "Go toolchain not installed, skipping Go fmt."; \
		fi \
	else echo "No go.mod yet, skipping Go fmt."; fi
	@if [ -f ui/package.json ]; then (cd ui && npm run format --if-present); else echo "No ui/package.json yet, skipping UI format."; fi

lint:
	@if [ -f go.mod ]; then \
		if ! command -v go >/dev/null 2>&1; then \
			echo "Go toolchain not installed, skipping Go lint."; \
		elif command -v golangci-lint >/dev/null 2>&1; then \
			golangci-lint run ./...; \
		else \
			echo "golangci-lint not installed, running go vet as fallback."; \
			go vet ./...; \
		fi \
	else echo "No go.mod yet, skipping Go lint."; fi
	@if [ -f ui/package.json ]; then (cd ui && npm run lint --if-present); else echo "No ui/package.json yet, skipping UI lint."; fi

test:
	@if [ -f go.mod ]; then \
		if command -v go >/dev/null 2>&1; then \
			go test ./...; \
		else \
			echo "Go toolchain not installed, skipping Go tests."; \
		fi \
	else echo "No go.mod yet, skipping Go tests."; fi
	@if [ -f ui/package.json ]; then (cd ui && npm test --if-present); else echo "No ui/package.json yet, skipping UI tests."; fi

race:
	@if [ -f go.mod ]; then \
		if command -v go >/dev/null 2>&1; then \
			go test -race ./...; \
		else \
			echo "Go toolchain not installed, skipping race tests."; \
		fi \
	else echo "No go.mod yet, skipping race tests."; fi

build:
	@if [ -f go.mod ]; then \
		if command -v go >/dev/null 2>&1; then \
			go build ./...; \
		else \
			echo "Go toolchain not installed, skipping Go build."; \
		fi \
	else echo "No go.mod yet, skipping Go build."; fi
	@if [ -f ui/package.json ]; then (cd ui && npm run build --if-present); else echo "No ui/package.json yet, skipping UI build."; fi

check: lint test

release-artifacts:
	@VERSION=$${VERSION:-0.0.0-dev}; \
	scripts/release/create_release_artifacts.sh "$$VERSION" "dist/release"
