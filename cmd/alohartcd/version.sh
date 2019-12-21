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
Conflicts: alohartcd-mendel, alohartcd-rpi0, alohartcd-rpi3
EOL
