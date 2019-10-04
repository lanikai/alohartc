TARGET ?= pizero
SYSROOT := $(CURDIR)/$(TARGET)

CFLAGS ?= -O3
ifeq ($(TARGET),pizero)
    HOST := arm-linux-gnueabihf
    export CFLAGS += -march=armv6 -mfpu=vfp -mfloat-abi=hard
endif
ifeq ($(TARGET),pi3)
    HOST := aarch64-linux-gnu
endif
export CFLAGS

# Create directories.
$(info $(shell mkdir -p src/ $(TARGET)/))

all: $(SYSROOT)/lib/libc.a $(SYSROOT)/lib/libnettle.a

MUSL_VERSION ?= 1.1.23
MUSL := musl-$(MUSL_VERSION)

src/$(MUSL).tar.gz:
	cd src/ && wget https://www.musl-libc.org/releases/$(MUSL).tar.gz

src/$(MUSL)/configure: src/$(MUSL).tar.gz
	cd src/ && tar xmf $(MUSL).tar.gz

$(SYSROOT)/lib/libc.a: src/$(MUSL)/configure
	mkdir -p $(SYSROOT)/build/musl/ && \
	cd $(SYSROOT)/build/musl/ && \
	../../../src/$(MUSL)/configure --host=$(HOST) --prefix=$(SYSROOT) && \
	make install

NETTLE_VERSION ?= 3.5.1
NETTLE := nettle-$(NETTLE_VERSION)

src/$(NETTLE).tar.gz:
	cd src/ && wget https://ftp.gnu.org/gnu/nettle/$(NETTLE).tar.gz

src/$(NETTLE)/configure: src/$(NETTLE).tar.gz
	cd src/ && tar xmf $(NETTLE).tar.gz

$(SYSROOT)/lib/libnettle.a: src/$(NETTLE)/configure
	mkdir -p $(SYSROOT)/build/nettle/ && \
	cd $(SYSROOT)/build/nettle/ && \
	../../../src/$(NETTLE)/configure \
	  --host=$(HOST) \
	  --prefix=$(SYSROOT) \
	  --disable-documentation \
	  CC=$(SYSROOT)/bin/musl-gcc && \
	make install
