PKGS := $(shell go list ./... | grep -v /vendor)

.PHONY: test
test:
	CGO_ENABLED=0 go test $(PKGS)

BINARY := dockhand-secrets-operator
VERSION := develop

ifdef CI_VERSION
VERSION := $(CI_VERSION)
endif

DEBUG ?= false
PLATFORMS := linux darwin
os = $(word 1, $@)

.DEFAULT_GOAL := local

local: test
	go build

.PHONY: $(PLATFORMS)
$(PLATFORMS):
	mkdir -p release/$(os)-amd64/$(VERSION)
	GOOS=$(os) GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-w -s -X main.Version=$(VERSION)" -o release/$(os)-amd64/$(VERSION)/$(BINARY)

arm64:
	mkdir -p release/linux-arm64/$(VERSION)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-w -s -X main.Version=$(VERSION)" -o release/linux-arm64/$(VERSION)/$(BINARY)

windows:
	mkdir -p release/windows-amd64/$(VERSION)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-w -s -X main.Version=$(VERSION)" -o release/windows-amd64/$(VERSION)/$(BINARY).exe

.PHONY: release
release: test windows linux darwin

.PHONY: linux-release
linux-release: linux

clean:
	rm -rf release/*
	rm -f $(BINARY)
