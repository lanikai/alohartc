TOP := $(shell git rev-parse --show-cdup)

default: alohartcd

.PHONY: generate
generate:
	$(MAKE) -C $(TOP) generate

# Command to build alohartcd binary
define build
    CGO_ENABLED=0 GOARCH=$(1) GOARM=$(2) GOOS=$(3) \
    go build -v -ldflags '-s -w' $(4) -o $@ .
endef


# Build executable
alohartcd:
	$(call build,$(GOARCH),$(GOARM),$(GOOS),-tags="alohartc mp4 rtsp v4l2")

# Build debian packages
deb: \
	deb/alohartcd-coral.deb \
	deb/alohartcd-rpi0.deb \
	deb/alohartcd-rpi3.deb \
	deb/alohartcd-tegra.deb

# Build for Google Coral EdgeTPU
deb/alohartcd-coral/usr/bin/alohartcd: generate
	$(call build,arm64,,linux,-tags="alohartc production v4l2")
deb/alohartcd-coral.deb: deb/alohartcd-coral/usr/bin/alohartcd
	dpkg-deb -b deb/alohartcd-coral .

# Build for Raspberry Pi Zero
deb/alohartcd-rpi0/usr/bin/alohartcd: generate
	$(call build,arm,6,linux,-tags="alohartc production v4l2")
deb/alohartcd-rpi0.deb: deb/alohartcd-rpi0/usr/bin/alohartcd
	dpkg-deb -b deb/alohartcd-rpi0 .

# Build for Raspberry Pi 3/3B+
deb/alohartcd-rpi3/usr/bin/alohartcd: generate
	$(call build,arm,7,linux,-tags="alohartc production v4l2")
deb/alohartcd-rpi3.deb: deb/alohartcd-rpi3/usr/bin/alohartcd
	dpkg-deb -b deb/alohartcd-rpi3 .

# Build executable for Tegra
deb/alohartcd-tegra/usr/bin/alohartcd: generate
	$(call build,arm64,,linux,-tags="alohartc production tegra")
deb/alohartcd-tegra.deb: deb/alohartcd-tegra/usr/bin/alohartcd
	dpkg-deb -b deb/alohartcd-tegra .

clean:
	find . -name 'alohartcd' -delete
	find . -name 'control' -delete

.PHONY: alohartcd clean deb
