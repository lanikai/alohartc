package media

/*
A MediaSource is a stream of media data that can have multiple consumers. The media
data is chunked into packets (which may represent discrete video frames, or
spans of multiple audio frames). Consumer functions register a Receiver, to
which the MediaSource sends packets. Each packet is delivered as a
*packet.SharedBuffer instance, which the consumer must process and then release.

If the MediaSource encounters an error, all receiver channels will be closed. The
Receiver's Err() function will return the reason for the interruption.

The MediaSource interface represents only the consumer-facing side of a media stream;
it makes no assumptions about how the data is produced. Nor does it describe the
nature of the data packets being delivered. It merely provides an interface for
common behavior between AudioSource and VideoSource, which extend MediaSource.

Example usage:

	var src MediaSource = ...
	r := src.AddReceiver(4)
	defer src.RemoveReceiver(r)
	for {
		buf, more := <-r.Buffers()
		if !more {
			// Process r.Err()
			break
		}
		// Do something with buf.Bytes(), then call buf.Release()
	}

*/
type MediaSource interface {
	// AddReceiver creates a new Receiver r, and starts passing incoming data
	// buffers to it. The source will not block sending to r, so the capacity
	// must be sufficient to keep up with the rate of incoming data. (In
	// particular, capacity must be > 0.) The Receiver channel may be closed if
	// the source is interrupted, in which case r.Err will be populated.
	//
	// Callers must ensure that the receiver is removed when processing is
	// complete (e.g. a defer statement immediately following AddReceiver()).
	Subscribe(capacity int) <-chan []byte

	// RemoveReceiver tells the source to stop passing data buffers to r. Upon
	// return, it is guaranteed r will not receive any more data.
	Unsubscribe(r <-chan []byte) error
}
