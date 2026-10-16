GOLANGCI_LINT_VERSION := v2.14.0
GOLANGCI_LINT := ./bin/golangci-lint
GORELEASER := github.com/goreleaser/goreleaser/v2@latest

.DEFAULT_GOAL := help
.PHONY: help build test lint smoke release-check

help: ## Show this help
	@grep -hE '^[a-z][a-zA-Z0-9_-]*:.*##' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

build: ## Build ./tmp/coin-book-mcp
	@mkdir -p tmp
	go build -trimpath -o tmp/coin-book-mcp .

test: ## Unit tests against a fake API, including the shared money vectors
	go test ./...

smoke: ## Contract check against the running stack: COINBOOK_TOKEN=coinbook_pat_… make smoke
	go test -count=1 -run Smoke -v ./internal/tools/

lint: $(GOLANGCI_LINT) ## golangci-lint
	$(GOLANGCI_LINT) run ./...

release-check: ## Validate .goreleaser.yaml and build a snapshot without publishing
	go run $(GORELEASER) release --snapshot --clean --skip=publish

$(GOLANGCI_LINT):
	@mkdir -p bin
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b ./bin $(GOLANGCI_LINT_VERSION)
