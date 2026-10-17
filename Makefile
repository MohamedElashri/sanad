.PHONY: build test race lint check fmt docs-build docs-serve sync-version package-backend test-native

export GOCACHE ?= $(CURDIR)/.gocache
export XDG_CACHE_HOME ?= $(CURDIR)/.cache
VERSION := $(shell cat VERSION)
COMMIT := $(shell git rev-parse HEAD)
STATICCHECK ?= $(CURDIR)/bin/staticcheck
STATICCHECK_VERSION := v0.8.2
STATICCHECK_STAMP := $(CURDIR)/bin/.staticcheck-$(STATICCHECK_VERSION)-$(shell go env GOVERSION)
LDFLAGS := -X github.com/MohamedElashri/sanad/internal/cli.version=$(VERSION) -X github.com/MohamedElashri/sanad/internal/cli.commit=$(COMMIT)

build:
	go build -ldflags '$(LDFLAGS)' -o bin/sanad ./cmd/sanad

test:
	go test ./...

test-native:
	go run ./scripts/package-backend --native --output .release-tools/native-backend
	SANAD_TEST_BACKEND=$(CURDIR)/.release-tools/native-backend go test ./internal/sanad -run TestNativeBackendContract -v

race:
	go test -race ./...

lint: $(STATICCHECK_STAMP)
	go vet ./...
	$(STATICCHECK) ./...

$(STATICCHECK_STAMP):
	GOBIN=$(CURDIR)/bin go install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
	touch $@

fmt:
	gofmt -w cmd internal scripts

check: test lint build

package-backend:
	go run ./scripts/package-backend

docs-build:
	nida build --site ./docs

docs-serve:
	nida serve --site ./docs

sync-version:
	scripts/sync-version
	npm run build --prefix action
