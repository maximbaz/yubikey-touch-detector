BIN := yubikey-touch-detector
VERSION := $(shell sed -nr 's/.*appVersion[ \t]*=[ \t]*"(.*)".*/\1/p' main.go)

PREFIX ?= /usr
LIB_DIR = $(DESTDIR)$(PREFIX)/lib
BIN_DIR = $(DESTDIR)$(PREFIX)/bin
SHARE_DIR = $(DESTDIR)$(PREFIX)/share

export CGO_CPPFLAGS := ${CPPFLAGS}
export CGO_CFLAGS := ${CFLAGS}
export CGO_CXXFLAGS := ${CXXFLAGS}
export CGO_LDFLAGS := ${LDFLAGS}
export GOFLAGS := -buildmode=pie -trimpath -ldflags=-linkmode=external

.PHONY: local
local: vendor build

.PHONY: run
run: local
	go run main.go

.PHONY: build
build: main.go detector/ notifier/
	go build -o $(BIN) main.go

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
dist: clean vendor build
	$(eval TMP := $(shell mktemp -d))
	mkdir "$(TMP)/$(BIN)-$(VERSION)"
	cp -r * "$(TMP)/$(BIN)-$(VERSION)"
	(cd "$(TMP)" && tar -cvzf "$(BIN)-$(VERSION)-src.tar.gz" "$(BIN)-$(VERSION)")

	mkdir "$(TMP)/$(BIN)-$(VERSION)-linux64"
	cp "$(BIN)" "$(BIN).service" "$(BIN).socket" LICENSE README.md "$(TMP)/$(BIN)-$(VERSION)-linux64"
	(cd "$(TMP)" && tar -cvzf "$(BIN)-$(VERSION)-linux64.tar.gz" "$(BIN)-$(VERSION)-linux64")

	mkdir -p dist
	mv "$(TMP)/$(BIN)-$(VERSION)"-*.tar.gz dist
	git archive -o "dist/$(BIN)-$(VERSION).tar.gz" --format tar.gz --prefix "$(BIN)-$(VERSION)/" "$(VERSION)"

	for file in dist/*; do \
	    gpg --detach-sign --armor "$$file"; \
	done

	rm -rf "$(TMP)"
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
