<h1 align="center">
  AlohaRTC
  <br>
</h1>
<h4 align="center">Real-time communication with cameras, natively within the browser</h4>
<p align="center">
  <a href="https://circleci.com/gh/lanikai/alohartc" alt="CircleCI"><img src="https://circleci.com/gh/lanikai/alohartc.svg?style=shield&circle-token=0bcc086c4c5c77ab6cfbdc85cb810f522ef7b8bd"></a>
</p>

## Setup

Set a pre-commit hook to `go fmt` code (see https://tip.golang.org/misc/git/pre-commit)


## Building

First, download dependencies:

    make get

To cross-compile for a Raspberry Pi Model 3B/3B+ (armv7-based architecture):

    make
    
To cross-compile for a Raspberry Pi Zero (armv6-based architecture):

    GOARM=6 make


## Quickstart

Build code as above, then transfer `alohacam` to Raspberry Pi and run. Open
http://<target>:8000 in browser. This should start a live video stream from
Raspberry Pi.
    
    
## Notes

Ensure camera is enabled on Raspberry Pi and that v4l2 module is loaded.

Modify `/etc/modules` as follows:

	# /etc/modules: kernel modules to load at boot time.
	#
	# This file contains the names of kernel modules that should be loaded
	# at boot time, one per line. Lines beginning with "#" are ignored.
	
	bcm2835-v4l2
	...

Modify `/boot/config.txt` as follows:

	boot/config.txt
	...
	start_x=1
	gpu_mem=128
