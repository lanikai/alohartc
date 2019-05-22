TOP := $(shell git rev-parse --show-cdup)

GIT_REVISION_ID := $(shell git describe --always --dirty)
BUILD_DATE := $(shell date)

# Default 'alohacam' target platform is Linux/ARMv7. Can be overridden on the
# command line. (Note: these default values are ignored for release builds.)
CGO_ENABLED ?= 0
GOARM ?= 7
GOARCH ?= arm
GOFLAGS ?=
GOOS ?= linux

# Command to build alohacam binary.
BUILDCMD = \
    CGO_ENABLED=$(CGO_ENABLED) GOARM=$(GOARM) GOARCH=$(GOARCH) GOFLAGS=$(GOFLAGS) GOOS=$(GOOS) \
    go build -v \
	-ldflags '-s -w -X main.GitRevisionId=$(GIT_REVISION_ID) -X "main.BuildDate=$(BUILD_DATE)"' \
	-o $@ .


.PHONY: all
all: alohacam

.PHONY: generate
generate:
	$(MAKE) -C $(TOP) generate

# PHONY because we let the Go build system decide if it's up-to-date.
.PHONY: alohacam
alohacam: generate
	$(BUILDCMD)


# Release version. Override on command line when building a release, e.g.
#     make release VERSION=0.5.6
VERSION ?= dev

RELEASE_TARGETS := \
    release/$(VERSION)/alohacam-linux-armv6 \
    release/$(VERSION)/alohacam-linux-armv7 \
    release/$(VERSION)/alohacam-linux-aarch64

# Set Go build environment for each release target.
$(RELEASE_TARGETS): CGO_ENABLED=0
$(RELEASE_TARGETS): GOFLAGS=-tags=oahu
release/$(VERSION)/alohacam-linux-%: GOOS=linux
release/$(VERSION)/alohacam-linux-armv%: GOARCH=arm
release/$(VERSION)/alohacam-linux-armv6: GOARM=6
release/$(VERSION)/alohacam-linux-armv7: GOARM=7
release/$(VERSION)/alohacam-linux-aarch64: GOARCH=arm64

.PHONY: $(RELEASE_TARGETS)
$(RELEASE_TARGETS): checktree generate
	$(BUILDCMD)

.PHONY: release
release: $(RELEASE_TARGETS)

.PHONY: checktree
.SILENT: checktree
ifneq ($(VERSION), dev)
BRANCH := $(shell git rev-parse --symbolic-full-name --abbrev-ref HEAD)
checktree:
	if [ $(BRANCH) != master ]; then \
	    echo "!! Versioned release binaries must be built from the master branch."; \
	    exit 1; \
	fi
	if ! git diff-index --exit-code --compact-summary HEAD; then \
	    echo "!! Versioned release binaries must be built from a clean working tree."; \
	    exit 1; \
	fi
endif


# Run a simple Python 3 download server, for development. For testing the
# Alohacam installer script.
download_server:
	python3 -m http.server 8086


# TODO: Add a make target to upload releases to S3.
