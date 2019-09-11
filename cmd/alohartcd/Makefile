TOP := $(shell git rev-parse --show-cdup)
VERSION := $(shell git describe --always --tags)

# Command to build alohartcd binary.
define buildcmd
    go build -v \
	-ldflags '-s -w -X main.GitRevisionId=$(VERSION)' \
	$(1) \
	-o $@ .
endef


.PHONY: all
all: alohartcd alohartcd-linux-armv6 alohartcd-linux-armv7 alohartcd-linux-aarch64

alohartcd%: export CGO_ENABLED=1
alohartcd-linux-%: export GOOS=linux
alohartcd-linux-armv%: export CC=arm-unknown-linux-gnueabi-gcc
alohartcd-linux-armv%: export GOARCH=arm
alohartcd-linux-armv%: export PKG_CONFIG_PATH=$(abspath $(dir $(shell which arm-unknown-linux-gnueabi-gcc))/../arm-unknown-linux-gnueabi/sysroot/usr/lib/pkgconfig)
alohartcd-linux-armv%: export PKG_CONFIG_LIBDIR=$(abspath $(dir $(shell which arm-unknown-linux-gnueabi-gcc))/../arm-unknown-linux-gnueabi/sysroot/usr/lib)
alohartcd-linux-armv6: export GOARM=6
alohartcd-linux-armv7: export GOARM=7
alohartcd-linux-aarch64: export CC=aarch64-unknown-linux-gnu-gcc
alohartcd-linux-aarch64: export GOARCH=arm64
alohartcd-linux-aarch64: export PKG_CONFIG_PATH=$(abspath $(dir $(shell which aarch64-unknown-linux-gnu-gcc))/../aarch64-unknown-linux-gnu/sysroot/usr/lib/pkgconfig)
alohartcd-linux-aarch64: export PKG_CONFIG_LIBDIR=$(abspath $(dir $(shell which aarch64-unknown-linux-gnu-gcc))/../aarch64-unknown-linux-gnu/sysroot/usr/lib)

.PHONY: alohartcd
alohartcd%: generate
	$(call buildcmd, -tags="mp4 rtsp v4l2")

.PHONY: generate
generate:
	$(MAKE) -C $(TOP) generate

.PHONY: clean
clean:
	rm -rf alohartcd alohartcd-linux-armv6 alohartcd-linux-armv7 alohartcd-linux-aarch64


###  BUILDS FOR DISTRIBUTION  ################################################

RELEASE_TARGETS := \
    release/$(VERSION)/alohartcd-linux-armv6 \
    release/$(VERSION)/alohartcd-linux-armv7 \
    release/$(VERSION)/alohartcd-linux-aarch64

# Set Go build environment for each release target.
$(RELEASE_TARGETS): export CGO_ENABLED=1
release/$(VERSION)/alohartcd-linux-%: export GOOS=linux
release/$(VERSION)/alohartcd-linux-armv%: export CC=arm-unknown-linux-gnueabi-gcc
release/$(VERSION)/alohartcd-linux-armv%: export GOARCH=arm
release/$(VERSION)/alohartcd-linux-armv%: export PKG_CONFIG_PATH=$(abspath $(dir $(shell which arm-unknown-linux-gnueabi-gcc))/../arm-unknown-linux-gnueabi/sysroot/usr/lib/pkgconfig)
release/$(VERSION)/alohartcd-linux-armv%: export PKG_CONFIG_LIBDIR=$(abspath $(dir $(shell which arm-unknown-linux-gnueabi-gcc))/../arm-unknown-linux-gnueabi/sysroot/usr/lib)
release/$(VERSION)/alohartcd-linux-armv6: export GOARM=6
release/$(VERSION)/alohartcd-linux-armv7: export GOARM=7
release/$(VERSION)/alohartcd-linux-aarch64: export CC=aarch64-unknown-linux-gnu-gcc
release/$(VERSION)/alohartcd-linux-aarch64: export GOARCH=arm64
release/$(VERSION)/alohartcd-linux-aarch64: export PKG_CONFIG_PATH=$(abspath $(dir $(shell which aarch64-unknown-linux-gnu-gcc))/../aarch64-unknown-linux-gnu/sysroot/usr/lib/pkgconfig)
release/$(VERSION)/alohartcd-linux-aarch64: export PKG_CONFIG_LIBDIR=$(abspath $(dir $(shell which aarch64-unknown-linux-gnu-gcc))/../aarch64-unknown-linux-gnu/sysroot/usr/lib)

.PHONY: $(RELEASE_TARGETS)
$(RELEASE_TARGETS): generate
	$(call buildcmd, -tags="release v4l2")

.PHONY: release
release: $(RELEASE_TARGETS)

# Helper targets for generating self-signed certificate
key.pem:
	openssl ecparam -out key.pem -name secp256r1 -genkey
cert.pem: key.pem
	openssl req -new -key key.pem -x509 -nodes -days 365 -out cert.pem -subj '/CN=localhost/O=Lanikai Labs LLC/C=US'
