.PHONY: build test race lint staticcheck bench fmt check docs-release-notes docs-build docs-serve clean

GOCACHE ?= $(CURDIR)/.gocache
NIDA ?= nida

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
	rm -f sanad
