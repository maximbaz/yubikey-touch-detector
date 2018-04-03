APP_NAME := yubikey-touch-detector

all: deps build

deps:
	dep ensure

build: main.go detector/ notifier/
	go build -o $(APP_NAME) main.go

clean:
	rm -f $(APP_NAME)
	rm -rf release

tarball: clean deps
	rm -rf /tmp/$(APP_NAME) /tmp/$(APP_NAME)-src.tar.gz
	cp -r ../$(APP_NAME) /tmp/$(APP_NAME)
	rm -rf /tmp/$(APP_NAME)/.git /tmp/$(APP_NAME)/tags
	(cd /tmp && tar -czf /tmp/$(APP_NAME)-src.tar.gz $(APP_NAME))
	mkdir -p release
	cp /tmp/$(APP_NAME)-src.tar.gz release/

release: tarball deps build
	mkdir -p release
	tar -czf release/$(APP_NAME).tar.gz $(APP_NAME) $(APP_NAME).service

sign: release
	for file in release/*; do \
		gpg --detach-sign "$$file"; \
	done
