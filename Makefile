BIN := yubikey-touch-detector
VERSION = 1.3.0

PREFIX ?= /usr
LIB_DIR = $(DESTDIR)$(PREFIX)/lib
BIN_DIR = $(DESTDIR)$(PREFIX)/bin
SHARE_DIR = $(DESTDIR)$(PREFIX)/share

GO_GCFLAGS := "all=-trimpath=${PWD}"
GO_ASMFLAGS := "all=-trimpath=${PWD}"
GO_LDFLAGS := "-extldflags ${LDFLAGS}"

.PHONY: build
build: main.go detector/ notifier/
	go build -ldflags $(GO_LDFLAGS) -gcflags $(GO_GCFLAGS) -asmflags $(GO_ASMFLAGS) -o $(BIN) main.go

.PHONY: clean
clean:
	rm -f "$(BIN)"
	rm -rf dist

.PHONY: dist
dist: clean build
	mkdir -p dist

	git archive -o "dist/$(BIN)-$(VERSION).tar.gz" --format tar.gz --prefix "$(BIN)-$(VERSION)/" "$(VERSION)"

	tar -cvzf "dist/$(BIN)-$(VERSION)-linux64.tar.gz" "$(BIN)" "$(BIN).service" LICENSE README.md

	for file in dist/*; do \
	    gpg --detach-sign --armor "$$file"; \
	done

	rm -f "dist/$(BIN)-$(VERSION).tar.gz"

.PHONY: install
install:
	install -Dm755 -t "$(BIN_DIR)/" $(BIN)
	install -Dm644 -t "$(LIB_DIR)/systemd/user" "$(BIN).service"
	install -Dm644 -t "$(SHARE_DIR)/licenses/$(BIN)/" LICENSE
	install -Dm644 -t "$(SHARE_DIR)/doc/$(BIN)/" README.md
