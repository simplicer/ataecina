# Ataecina — every development command goes through this Makefile.
SHELL := /bin/bash
GO ?= go
VERSION := $(shell tr -d '[:space:]' < VERSION)

# Reproducible, static builds (AGENTS.md, "Tooling").
export CGO_ENABLED := 0
BUILD_FLAGS := -trimpath -buildvcs=false -ldflags "-s -w -X main.version=$(VERSION)"

# Build-time tools, pinned and never linked (ADR 0008, ADR 0009).
GOVULNCHECK := golang.org/x/vuln/cmd/govulncheck@v1.9.0
NFPM := $(GO) run github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.48.0

PACKAGE_ARCHES := amd64 arm64
PACKAGE_FORMATS := deb rpm archlinux
# This machine's architecture, for `make deb`.
HOST_ARCH := $(shell $(GO) env GOARCH)
DEB_FILE = dist/ataecina_$(shell bin/package-version deb)-1_$(HOST_ARCH).deb
# A command-line PREFIX=… overrides this; a PREFIX in the environment does not.
PREFIX := $(HOME)/.local

BUILD_DIR := var/build
E2E_DIR := var/e2e
COVERAGE_DIR := var/coverage
APPLICATION_PACKAGES = $(shell $(GO) list ./internal/... | grep '/application$$')

# The Plasma widget (ADR 0007), staged in var/build/plasmoid with its
# compiled translations.
WIDGET_ID := org.simplicer.ataecina
WIDGET_SOURCE := share/plasma/plasmoids/$(WIDGET_ID)
WIDGET_BUILD := var/build/plasmoid/$(WIDGET_ID)
QML_IMPORTS := /usr/lib/x86_64-linux-gnu/qt6/qml /usr/lib/qt6/qml /usr/lib64/qt6/qml
QMLLINT ?= $(firstword $(shell command -v qmllint qmllint6 /usr/lib/qt6/bin/qmllint /usr/lib64/qt6/bin/qmllint 2>/dev/null))

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
	./internal/allowances/infrastructure/commandcodeapi:FuzzDecodeCredits \
	./internal/allowances/infrastructure/ollamaapi:FuzzDecodeUsage \
	./internal/allowances/infrastructure/orcarouterapi:FuzzDecodeSubscription \
	./internal/allowances/infrastructure/cloudflareapi:FuzzDecodeNeurons \
	./internal/allowances/infrastructure/copilotgh:FuzzDecodeReport \
	./internal/allowances/infrastructure/alibabacli:FuzzDecodeUsage \
	./internal/allowances/infrastructure/heimdallplans:FuzzDecodePlans
FUZZ_TIME ?= 30s

.PHONY: all check fmt-check vet unit coverage integration acceptance bench-smoke widget-lint vulncheck build widget check-version e2e fuzz package deb install-deb uninstall-deb install-user uninstall-user clean

# `make` alone compiles the binary and stages the widget.
.DEFAULT_GOAL := all
all: build widget

check: fmt-check vet unit coverage integration acceptance bench-smoke widget-lint vulncheck build widget check-version

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

# qmllint is optional on a developer machine: tests/widget always checks
# the widget statically, and msgfmt always checks its translations.
widget-lint:
	@if [ -n "$(QMLLINT)" ]; then \
		$(QMLLINT) $(addprefix -I ,$(wildcard $(QML_IMPORTS))) $(WIDGET_SOURCE)/contents/*/*.qml; \
	else \
		echo "warning: qmllint not found; the widget was only checked by tests/widget"; \
	fi
	@for po in $(WIDGET_SOURCE)/translate/*.po; do msgfmt --check -o /dev/null "$$po" || exit 1; done

vulncheck:
	$(GO) run $(GOVULNCHECK) ./...

build:
	$(GO) build $(BUILD_FLAGS) -o $(BUILD_DIR)/ataecina ./cmd/ataecina

widget:
	rm -rf var/build/plasmoid
	@mkdir -p var/build/plasmoid
	cp -r $(WIDGET_SOURCE) $(WIDGET_BUILD)
	rm -r $(WIDGET_BUILD)/translate
	@for po in $(WIDGET_SOURCE)/translate/*.po; do \
		locale=$(WIDGET_BUILD)/contents/locale/$$(basename "$$po" .po)/LC_MESSAGES; \
		mkdir -p "$$locale"; \
		msgfmt --check -o "$$locale/plasma_applet_$(WIDGET_ID).mo" "$$po" || exit 1; \
	done

check-version:
	bin/check-version

e2e: build widget
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

# .deb, .rpm and Arch packages of each architecture in dist/ (ADR 0008).
package: widget
	@set -e; for arch in $(PACKAGE_ARCHES); do \
		echo "go build GOARCH=$$arch"; \
		GOARCH=$$arch $(GO) build $(BUILD_FLAGS) -o var/build/package/$$arch/ataecina ./cmd/ataecina; \
	done
	NFPM="$(NFPM)" PACKAGE_FORMATS="$(PACKAGE_FORMATS)" SOURCE_DATE_EPOCH=$$(git log -1 --format=%ct) \
		bin/build-packages $(PACKAGE_ARCHES)
	PACKAGE_FORMATS="$(PACKAGE_FORMATS)" bin/check-packages $(PACKAGE_ARCHES)

# Only the .deb of this machine, verified and test-installed in a fake root.
deb:
	$(MAKE) package PACKAGE_ARCHES=$(HOST_ARCH) PACKAGE_FORMATS=deb
	@echo "deb: $(DEB_FILE)"

# Installs (or reinstalls) that .deb system-wide with apt; asks for sudo.
install-deb: deb
	sudo apt install --reinstall ./$(DEB_FILE)
	@echo "next, as yourself: ataecina init, then see README \"First set-up, step by step\" (subscriptions, keys, timer, widget or tray)"

uninstall-deb:
	sudo apt remove ataecina

# Into ~/.local (or PREFIX=…, DESTDIR=…); units are installed, not enabled.
USER_INSTALL = --prefix $(PREFIX) --systemd-user-dir $(PREFIX)/share/systemd/user $(if $(DESTDIR),--destdir $(DESTDIR))

install-user: build widget
	bin/install-files $(USER_INSTALL)
	@echo "next: systemctl --user daemon-reload && systemctl --user enable --now ataecina-collect.timer"

uninstall-user:
	bin/install-files --uninstall $(USER_INSTALL)

clean:
	rm -rf $(BUILD_DIR) $(E2E_DIR) $(COVERAGE_DIR) dist
