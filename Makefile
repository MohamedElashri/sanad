.PHONY: help build test race lint staticcheck bench fmt check docs-release-notes docs-build docs-serve clean

.DEFAULT_GOAL := help

GOCACHE ?= $(CURDIR)/.gocache
NIDA ?= nida

help:
	@printf '%s\n' 'Sanad development targets:'
	@printf '  %-20s %s\n' 'make build' 'Build all Go packages'
	@printf '  %-20s %s\n' 'make test' 'Run the test suite'
	@printf '  %-20s %s\n' 'make race' 'Run tests with the race detector'
	@printf '  %-20s %s\n' 'make lint' 'Run go vet'
	@printf '  %-20s %s\n' 'make staticcheck' 'Run Staticcheck'
	@printf '  %-20s %s\n' 'make bench' 'Run workflow extraction benchmark'
	@printf '  %-20s %s\n' 'make fmt' 'Format Go sources'
	@printf '  %-20s %s\n' 'make check' 'Format, lint, test, and build'
	@printf '  %-20s %s\n' 'make docs-build' 'Generate release notes and build docs'
	@printf '  %-20s %s\n' 'make docs-serve' 'Generate release notes and serve docs'
	@printf '  %-20s %s\n' 'make clean' 'Remove local build, docs, and cache artifacts'

build:
	GOCACHE=$(GOCACHE) go build ./...

test:
	GOCACHE=$(GOCACHE) go test ./...

race:
	GOCACHE=$(GOCACHE) go test -race ./...

lint:
	GOCACHE=$(GOCACHE) go vet ./...

staticcheck:
	GOCACHE=$(GOCACHE) go run honnef.co/go/tools/cmd/staticcheck@latest ./...

bench:
	GOCACHE=$(GOCACHE) go test -run '^$$' -bench BenchmarkExtractUsesFromLargeWorkflow ./internal/workflow

fmt:
	gofmt -w cmd internal

check: fmt lint test build

docs-release-notes:
	GOCACHE=$(GOCACHE) go run ./scripts/generate_release_notes.go

docs-build: docs-release-notes
	$(NIDA) build --site ./docs

docs-serve: docs-release-notes
	$(NIDA) serve --site ./docs

clean:
	$(RM) -r sanad dist .gocache .gomodcache docs/public docs/content/release-notes.md
