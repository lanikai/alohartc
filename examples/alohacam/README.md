Demonstrates send-only video from WebRTC native peer. Uses a built-in signaling
server.

To cross-compile (e.g. for aarch64 / arm64):

    CC=aarch64-unknown-linux-gnu-gcc GOARCH=arm64 GOFLAGS="-tags=localonly" make
