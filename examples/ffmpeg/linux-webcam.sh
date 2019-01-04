#/bin/bash
#
# Stream from v4l2-enabled webcam with minimal latency.

SOURCE=/dev/video0
DEST=${1:-video.264}

if ! [ -p $DEST ]; then
    rm -f $DEST
    mkfifo $DEST
fi

ffmpeg -i $SOURCE \
    -f h264 -profile:v baseline -level 3.1 -pix_fmt yuv420p -preset ultrafast -tune zerolatency \
    -y $DEST
