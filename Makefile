.PHONY: help build test race lint staticcheck bench fmt check docs-release-notes docs-build docs-serve clean

.DEFAULT_GOAL := help

GOCACHE ?= $(CURDIR)/.gocache
GOMODCACHE ?= $(CURDIR)/.gomodcache
BIN_DIR ?= $(CURDIR)/bin
NIDA ?= nida

help:
	@printf '%s\n' 'Sanad development targets:'
	@printf '  %-20s %s\n' 'make build' 'Build the sanad binary into ./bin'
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
	mkdir -p $(BIN_DIR)
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go build -o $(BIN_DIR)/sanad ./cmd/sanad

test:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go test ./...

race:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go test -race ./...

lint:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go vet ./...

staticcheck:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go run honnef.co/go/tools/cmd/staticcheck@latest ./...

bench:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go test -run '^$$' -bench BenchmarkExtractUsesFromLargeWorkflow ./internal/workflow

fmt:
	gofmt -w cmd internal

check: fmt lint test build

docs-release-notes:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) go run ./scripts/generate_release_notes.go

docs-build: docs-release-notes
	$(NIDA) build --site ./docs

docs-serve: docs-release-notes
	$(NIDA) serve --site ./docs

clean:
	$(RM) -r sanad bin dist .gocache docs/public docs/content/release-notes.md
