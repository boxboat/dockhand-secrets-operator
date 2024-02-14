comma := ,
space := $(subst ,, )

BINARY ?= dockhand-secrets-operator
VERSION ?= develop
REGISTRY ?= docker.io
DOCKER_TARGET ?= release
PKGS ?= $(shell go list ./... | grep -v /vendor)
DEBUG ?= false
PROGRESS ?= plain
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
OSES ?= linux darwin windows
ARCHES ?= amd64 arm64
PLATFORM_FILTERS ?= windows-arm64
DOCKER_PLATFORM_FILTERS ?= windows/% darwin/%
RELEASE_TARGET_FILTERS ?= $(foreach filter, $(PLATFORM_FILTERS), release-$(subst /,-,$(filter)))
RELEASE_TARGETS ?= $(strip $(filter-out $(RELEASE_TARGET_FILTERS), $(foreach arch, $(ARCHES), $(foreach os, $(OSES), release-$(os)-$(arch)))))
DOCKER_PLATFORMS ?= $(subst $(space),$(comma),$(strip $(filter-out $(DOCKER_PLATFORM_FILTERS), $(foreach arch, $(ARCHES), $(foreach os, $(OSES), $(os)/$(arch))))))

ifdef CI_VERSION
VERSION := $(CI_VERSION)
endif

target = $(subst -,$(space),$(@))
os = $(word 2, $(target))
arch = $(word 3, $(target))

.PHONY: build test release $(RELEASE_TARGETS) docker

.DEFAULT_GOAL := build

generate:
	go generate

build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build \
	-ldflags="-w -s -X main.Version=$(VERSION)" \
	-o bin/

test:
	CGO_ENABLED=0 go test $(PKGS)

release: $(RELEASE_TARGETS)
$(RELEASE_TARGETS):
	mkdir -p ./release/$(os)-$(arch)/$(VERSION)
	GOOS=$(os) GOARCH=$(arch) CGO_ENABLED=0 go build \
    		-ldflags="-w -s -X main.Version=$(VERSION)" \
    		-o ./release/$(os)-$(arch)/$(VERSION)/

docker:
	docker buildx build \
		--target $(DOCKER_TARGET) \
		--platform $(DOCKER_PLATFORMS) \
		--build-arg VERSION=$(VERSION) \
		-t $(REGISTRY)/boxboat/$(BINARY):$(VERSION) \
		--push \
		--progress $(PROGRESS) \
		.

clean:
	rm -rf bin release
