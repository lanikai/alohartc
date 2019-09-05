Demonstrates send-only video from WebRTC native peer. Uses a built-insignaling
server.

To cross-compile (e.g. for aarch64 / arm64):

    CC=aarch64-unknown-linux-gnu-gcc GOARCH=arm64 GOFLAGS="-tags=nomqtt" make

To cross-compile for an armv6:

    PKG_CONFIG_PATH=$HOME/x-tools/arm-unknown-linux-gnueabi/arm-unknown-linux-gnueabi/sysroot/usr/lib/pkgconfig CC=arm-unknown-linux-gnueabi-gcc GOARCH=arm GOARM=6 GOFLAGS="-tags=nomqtt" make

To generate a self-signed certificate for local audio development (Chrome
requires HTTPS for `getUserMedia`):

    openssl req -x509 -newkey ecdsa -keyout key.pem -out cert.pem
