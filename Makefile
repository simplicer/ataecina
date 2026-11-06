# Ataecina — every development command goes through this Makefile.
SHELL := /bin/bash
GO ?= go
VERSION := $(shell tr -d '[:space:]' < VERSION)

# Reproducible, static builds (AGENTS.md, "Tooling").
export CGO_ENABLED := 0
BUILD_FLAGS := -trimpath -buildvcs=false -ldflags "-s -w -X main.version=$(VERSION)"

# Build-time tool, pinned and never linked (ADR 0009).
GOVULNCHECK := golang.org/x/vuln/cmd/govulncheck@v1.9.0

BUILD_DIR := var/build
E2E_DIR := var/e2e
COVERAGE_DIR := var/coverage
APPLICATION_PACKAGES = $(shell $(GO) list ./internal/... | grep '/application$$')

# package:FuzzTarget pairs explored by `make fuzz`; their seed corpus runs in every `go test`.
FUZZ_TARGETS := ./internal/allowances/infrastructure/configfile:FuzzDecode \
	./internal/allowances/infrastructure/codexlogs:FuzzDecodeLine \
	./internal/allowances/infrastructure/codexappserver:FuzzDecodeRateLimits \
	./internal/allowances/infrastructure/claudestatusline:FuzzDecodeRateLimits \
	./internal/allowances/infrastructure/opencodezen:FuzzDecodeSpend \
	./internal/allowances/infrastructure/openrouterapi:FuzzDecodeKey \
	./internal/allowances/infrastructure/vercelapi:FuzzDecodeCredits \
	./internal/allowances/infrastructure/zaiapi:FuzzDecodeQuota \
	./internal/allowances/infrastructure/minimaxapi:FuzzDecodeRemains \
	./internal/allowances/infrastructure/commandcodeapi:FuzzDecodeCredits
FUZZ_TIME ?= 30s

.PHONY: check fmt-check vet unit coverage integration acceptance bench-smoke vulncheck build check-version e2e fuzz clean

check: fmt-check vet unit coverage integration acceptance bench-smoke vulncheck build check-version

fmt-check:
	@unformatted=$$(gofmt -l cmd internal tests); \
	if [ -n "$$unformatted" ]; then echo "gofmt is needed on:"; echo "$$unformatted"; exit 1; fi

vet:
	$(GO) vet ./...
	$(GO) vet -tags integration ./...
	$(GO) vet -tags acceptance ./...
	$(GO) vet -tags e2e ./...

unit:
	$(GO) test -count=1 ./...

coverage:
	@mkdir -p $(COVERAGE_DIR)
	$(GO) test -count=1 -coverprofile=$(COVERAGE_DIR)/application.out $(APPLICATION_PACKAGES)
	bin/check-coverage $(COVERAGE_DIR)/application.out

integration:
	$(GO) test -count=1 -tags integration ./internal/...

acceptance:
	$(GO) test -count=1 -tags acceptance ./cmd/...

bench-smoke:
	$(GO) test -count=1 -tags integration -run '^$$' -bench . -benchtime 1x ./internal/...

vulncheck:
	$(GO) run $(GOVULNCHECK) ./...

build:
	$(GO) build $(BUILD_FLAGS) -o $(BUILD_DIR)/ataecina ./cmd/ataecina

check-version:
	bin/check-version

e2e: build
	$(GO) build -tags e2e $(BUILD_FLAGS) -o $(E2E_DIR)/ataecina ./cmd/ataecina
	ATAECINA_E2E_BINARY=$(abspath $(E2E_DIR)/ataecina) \
	ATAECINA_PRODUCTION_BINARY=$(abspath $(BUILD_DIR)/ataecina) \
	$(GO) test -count=1 -tags e2e ./tests/e2e/...

fuzz:
	@set -e; for target in $(FUZZ_TARGETS); do \
		package=$${target%%:*}; name=$${target##*:}; \
		echo "fuzz $$name ($(FUZZ_TIME))"; \
		$(GO) test -run '^$$' -fuzz "^$$name$$" -fuzztime $(FUZZ_TIME) $$package; \
	done

clean:
	rm -rf $(BUILD_DIR) $(E2E_DIR) $(COVERAGE_DIR)
