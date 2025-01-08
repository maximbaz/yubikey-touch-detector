BIN := yubikey-touch-detector
VERSION := $(shell git describe --tags)

PREFIX ?= /usr
LIB_DIR = $(DESTDIR)$(PREFIX)/lib
BIN_DIR = $(DESTDIR)$(PREFIX)/bin
SHARE_DIR = $(DESTDIR)$(PREFIX)/share

export CGO_CPPFLAGS := ${CPPFLAGS}
export CGO_CFLAGS := ${CFLAGS}
export CGO_CXXFLAGS := ${CXXFLAGS}
export CGO_LDFLAGS := ${LDFLAGS}
export GOFLAGS := -buildmode=pie -trimpath -ldflags=-linkmode=external

.PHONY: run
run: build
	./$(BIN)

.PHONY: build
build: main.go detector/ notifier/ yubikey-touch-detector.1
	go build -o $(BIN) main.go

yubikey-touch-detector.1: yubikey-touch-detector.1.scd
	scdoc < '$<' > '$@'

.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor

.PHONY: clean
clean:
	rm -f "$(BIN)"
	rm -rf dist
	rm -rf vendor

.PHONY: dist
dist: clean vendor
	mkdir -p dist
	git archive -o "dist/$(BIN)-$(VERSION).tar.gz" --format tar.gz --prefix "$(BIN)-$(VERSION)/" "$(VERSION)"
	git archive -o "dist/$(BIN)-$(VERSION)-src.tar.gz" --format tar.gz $$(find vendor -type f -printf '--prefix=$(BIN)-$(VERSION)/%h/ --add-file=%p ') --prefix "$(BIN)-$(VERSION)/" "$(VERSION)"

	for file in dist/*; do \
	    gpg --detach-sign --armor "$$file"; \
	done

	rm -f "dist/$(BIN)-$(VERSION).tar.gz"

.PHONY: install
install:
	install -Dm755 -t "$(BIN_DIR)/" $(BIN)
	install -Dm644 -t "$(LIB_DIR)/systemd/user" "$(BIN).service"
	install -Dm644 -t "$(LIB_DIR)/systemd/user" "$(BIN).socket"
	install -Dm644 -t "$(SHARE_DIR)/icons/hicolor/128x128/apps/" yubikey-touch-detector.png
	install -Dm644 -t "$(SHARE_DIR)/licenses/$(BIN)/" LICENSE
	install -Dm644 -t "$(SHARE_DIR)/doc/$(BIN)/" README.md
	install -Dm644 -t "$(SHARE_DIR)/doc/$(BIN)/" service.conf.example
	install -Dm644 -t "$(SHARE_DIR)/man/man1/" yubikey-touch-detector.1
