TOP := $(shell git rev-parse --show-toplevel)

TARGET ?= host
CFLAGS ?= -O3

ifeq ($(TARGET),pizero)
    # Build for Raspberry Pi Zero running Raspbian.
    # See https://www.raspbian.org/RaspbianFAQ#What_compilation_options_should_be_set_Raspbian_code.3F
    CC = $(TOP)/xbuild/pizero/bin/musl-gcc
    CFLAGS += -march=armv6 -mfpu=vfp -mfloat-abi=hard
    STATIC = -static
    export GOARCH=arm
    export GOARM=6
    export GOOS=linux
endif

all: bench-stdlib-$(TARGET) bench-nettle-$(TARGET)
.PHONY: all

bench-stdlib-$(TARGET):
	go build -tags=aes_stdlib -o $(@) .
.PHONY: bench-stdlib-$(TARGET)

bench-nettle-$(TARGET):
	CC=$(CC) CGO_ENABLED=1 CGO_CFLAGS="$(CFLAGS)" CGO_LDFLAGS="-lnettle $(STATIC)" \
		go build -tags=aes_nettle -o $(@) .
.PHONY: bench-nettle-$(TARGET)

clean:
	rm bench-{stdlib,nettle}-*
.PHONY: clean
