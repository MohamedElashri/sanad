.PHONY: build test race lint staticcheck bench fmt check clean

GOCACHE ?= $(CURDIR)/.gocache

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

clean:
	rm -f sanad
