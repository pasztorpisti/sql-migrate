SHELL = /bin/sh -e
GO_TAGS ?=
BIN_DIR ?= $(GOPATH)/bin
DEP_VERSION = v0.3.2

.PHONY: check_go_fmt all deps test unit-test integration-test build clean

all: check_go_fmt deps test build

deps:
	@if [ $$(uname) = "Linux" ]; then \
		echo "Downloading dep $(DEP_VERSION) ..."; \
		wget -qO $(BIN_DIR)/dep https://github.com/golang/dep/releases/download/$(DEP_VERSION)/dep-linux-amd64; \
		chmod +x $(BIN_DIR)/dep; \
	elif [ $$(uname) = "Darwin" ]; then \
		echo "Downloading dep $(DEP_VERSION) ..."; \
		curl -sLo $(BIN_DIR)/dep https://github.com/golang/dep/releases/download/$(DEP_VERSION)/dep-darwin-amd64; \
		chmod +x $(BIN_DIR)/dep; \
	else \
		>&2 echo "Unsupported OS: $$(uname)"; \
		exit 1; \
	fi
	dep ensure -vendor-only

test: unit-test integration-test

unit-test:
	go test -tags="$(GO_TAGS)" -v ./...

integration-test:
	integration-test/run_all_tests.sh

build: clean
	mkdir build

	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a \
		-ldflags "-extldflags \"-static\" -X main.version=$${VERSION:-$${TRAVIS_TAG-}} -X main.gitHash=$${GIT_HASH:-$${TRAVIS_COMMIT-}} -X main.buildDate=$$(date -u +%F)" \
		-tags=$(GO_TAGS) -o build/sql-migrate github.com/pasztorpisti/sql-migrate
	cd build \
		&& zip -q sql-migrate-linux-amd64.zip sql-migrate \
		&& shasum -a 256 sql-migrate sql-migrate-linux-amd64.zip > sql-migrate-linux-amd64.zip.sha256 \
		&& rm sql-migrate

	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -a \
		-ldflags "-extldflags \"-static\" -X main.version=$${VERSION:-$${TRAVIS_TAG-}} -X main.gitHash=$${GIT_HASH:-$${TRAVIS_COMMIT-}} -X main.buildDate=$$(date -u +%F)" \
		-tags=$(GO_TAGS) -o build/sql-migrate github.com/pasztorpisti/sql-migrate
	cd build \
		&& zip -q sql-migrate-darwin-amd64.zip sql-migrate \
		&& shasum -a 256 sql-migrate sql-migrate-darwin-amd64.zip > sql-migrate-darwin-amd64.zip.sha256 \
		&& rm sql-migrate

	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -a \
		-ldflags "-extldflags \"-static\" -X main.version=$${VERSION:-$${TRAVIS_TAG-}} -X main.gitHash=$${GIT_HASH:-$${TRAVIS_COMMIT-}} -X main.buildDate=$$(date -u +%F)" \
		-tags=$(GO_TAGS) -o build/sql-migrate.exe github.com/pasztorpisti/sql-migrate
	cd build \
		&& zip -q sql-migrate-windows-amd64.zip sql-migrate.exe \
		&& shasum -a 256 sql-migrate.exe sql-migrate-windows-amd64.zip > sql-migrate-windows-amd64.zip.sha256 \
		&& rm sql-migrate.exe

clean:
	rm -rf build

check_go_fmt:
	@if [ -n "$$(gofmt -d $$(find . -name '*.go' -not -path './vendor/*'))" ]; then \
		>&2 echo "The .go sources aren't formatted. Please format them with 'go fmt'."; \
		exit 1; \
	fi
