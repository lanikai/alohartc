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
    -f h264 -profile:v baseline -level 4.0 -preset ultrafast \
    -y $DEST
