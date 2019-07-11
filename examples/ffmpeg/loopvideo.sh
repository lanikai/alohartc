#!/bin/bash
#
# Loop the given source video repeatedly. Play it in real-time to simulate a live stream.

SOURCE=$1
DEST=${2:-video.264}

if ! [ -p $DEST ]; then
    rm -f $DEST
    mkfifo $DEST
fi

ffmpeg -re -stream_loop -1 -i $SOURCE \
    -f h264 -profile:v baseline -level 3.1 -preset superfast -x264opts keyint=25 \
    -b:v 2M -maxrate 2M -bufsize 2M \
    -y $DEST
