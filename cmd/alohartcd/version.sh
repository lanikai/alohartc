#!/bin/bash

# For use with go:generate in creating a version file

version=$(git describe --always --tags)

# Generate version function for Go code
cat >version.go <<EOL
package main

import "fmt"

const versionString = \`alohartcd ${version}
Copyright 2019 Lanikai Labs LLC. All rights reserved.\`

func version() {
	fmt.Println(versionString)
}
EOL

package=alohartcd
packagetype=deb
section=video
installedsize=2048
priority=optional
homepage=https://alohartc.com

# Generate debian control file for Google Coral EdgeTPU devices
mkdir -p deb/alohartcd-coral/DEBIAN
cat >deb/alohartcd-coral/DEBIAN/control <<EOL
Package: ${package}-coral
Package-Type: ${packagetype}
Version: ${version}
Maintainer: Lanikai Labs LLC <aloha@lanikailabs.com>
Description: AlohaRTC daemon for Google Coral EdgeTPU devices
 AlohaRTC is a serverless platform for real-time video communication between
 connected devices and web as well as mobile apps. This package installs the
 alohartcd daemon client for Google Coral EdgeTPU devices.
Section: ${section}
Priority: ${priority}
Installed-Size: ${installedsize}
Architecture: arm64
Homepage: ${homepage}
Depends: gstreamer1.0-tools, openssl
Conflicts: alohartcd-tegra, alohartcd-rpi0, alohartcd-rpi3
EOL

# Generate debian control file for tegra devices
mkdir -p deb/alohartcd-tegra/DEBIAN
cat >deb/alohartcd-tegra/DEBIAN/control <<EOL
Package: ${package}-tegra
Package-Type: ${packagetype}
Version: ${version}
Maintainer: Lanikai Labs LLC <aloha@lanikailabs.com>
Description: AlohaRTC daemon for Nvidia Tegra based devices
 AlohaRTC is a serverless platform for real-time video communication between
 connected devices and web as well as mobile apps. This package installs the
 alohartcd daemon client for Nvidia Tegra based devices, such as the
 Jetson Nano, Jetson TX2, Jetson Xavier, DRIVE PX, and similar devices.
Section: ${section}
Priority: ${priority}
Installed-Size: ${installedsize}
Architecture: arm64
Homepage: ${homepage}
Depends: gstreamer1.0-tools, openssl
Conflicts: alohartcd-coral, alohartcd-rpi0, alohartcd-rpi3
EOL

# Generate debian control file for Raspberry Pi 0
mkdir -p deb/alohartcd-rpi0/DEBIAN
cat >deb/alohartcd-rpi0/DEBIAN/control <<EOL
Package: ${package}-rpi0
Package-Type: ${packagetype}
Version: ${version}
Maintainer: Lanikai Labs LLC <aloha@lanikailabs.com>
Description: AlohaRTC daemon for Raspberry Pi Zero
 AlohaRTC is a serverless platform for real-time video communication between
 connected devices and web as well as mobile apps. This package installs the
 alohartcd daemon client for the Raspberry Pi Zero.
Section: ${section}
Priority: ${priority}
Installed-Size: ${installedsize}
Architecture: armhf
Homepage: ${homepage}
Conflicts: alohartcd-coral, alohartcd-rpi3, alohartcd-tegra
EOL

# Generate debian control file for Raspberry Pi 3/3B+
mkdir -p deb/alohartcd-rpi3/DEBIAN
cat >deb/alohartcd-rpi3/DEBIAN/control <<EOL
Package: ${package}-rpi3
Package-Type: ${packagetype}
Version: ${version}
Maintainer: Lanikai Labs LLC <aloha@lanikailabs.com>
Description: AlohaRTC daemon for Raspberry Pi 3/3B+
 AlohaRTC is a serverless platform for real-time video communication between
 connected devices and web as well as mobile apps. This package installs the
 alohartcd daemon client for the Raspberry Pi 3/3B+.
Section: ${section}
Priority: ${priority}
Installed-Size: ${installedsize}
Architecture: armhf
Homepage: ${homepage}
Conflicts: alohartcd-coral, alohartcd-rpi0, alohartcd-rpi3
EOL
