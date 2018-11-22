# WebRTC

Go package implementing a WebRTC native client


## Setup

Set a pre-commit hook to `go fmt` code (see https://tip.golang.org/misc/git/pre-commit)


## Building

To cross-compile for an armv7-based architecture, such as the Raspberry Pi Model 3B+:

    make


## Quickstart

Build code as above, then transfer `examples/demo/demo` to Raspberry Pi and run.
Open http://<target>:8000 in browser. This should start a live video stream from
Raspberry Pi.
    
    
## Notes

Ensure camera is enabled on Raspberry Pi and that v4l2 module is loaded.

Modify `/etc/modules` as follows:
```/etc/modules
# /etc/modules: kernel modules to load at boot time.
#
# This file contains the names of kernel modules that should be loaded
# at boot time, one per line. Lines beginning with "#" are ignored.

bcm2835-v4l2
...
```

Modify `/boot/config.txt` as follows:
```/boot/config.txt
...
start_x=1
gpu_mem=128
```
