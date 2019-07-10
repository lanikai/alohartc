// Copyright 2019 Lanikai Labs. All rights reserved.

package srtp

import "errors"

var (
	errMalformedPacket    = errors.New("malformed packet")
	errUnsupportedVersion = errors.New("unsupported version")
)
