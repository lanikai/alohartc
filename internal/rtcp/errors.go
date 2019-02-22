// MIT License
//
// Copyright (c) 2018 Pions
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package rtcp

import "errors"

var (
	errInvalidTotalLost = errors.New("rtcp: invalid total lost count")
	errInvalidHeader    = errors.New("rtcp: invalid header")
	errTooManyReports   = errors.New("rtcp: too many reports")
	errTooManyChunks    = errors.New("rtcp: too many chunks")
	errTooManySources   = errors.New("rtcp: too many sources")
	errPacketTooShort   = errors.New("rtcp: packet too short")
	errWrongType        = errors.New("rtcp: wrong packet type")
	errSDESTextTooLong  = errors.New("rtcp: sdes must be < 255 octets long")
	errSDESMissingType  = errors.New("rtcp: sdes item missing type")
	errReasonTooLong    = errors.New("rtcp: reason must be < 255 octets long")
	errBadVersion       = errors.New("rtcp: invalid packet version")
)
