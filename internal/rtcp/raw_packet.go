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

// RawPacket represents an unparsed RTCP packet. It's returned by Unmarshal when
// a packet with an unknown type is encountered.
type RawPacket []byte

// Marshal encodes the packet in binary.
func (r RawPacket) Marshal() ([]byte, error) {
	return r, nil
}

// Unmarshal decodes the packet from binary.
func (r *RawPacket) Unmarshal(b []byte) error {
	if len(b) < (headerLength) {
		return errPacketTooShort
	}
	*r = b

	var h Header
	return h.Unmarshal(b)
}

// Header returns the Header associated with this packet.
func (r RawPacket) Header() Header {
	var h Header
	if err := h.Unmarshal(r); err != nil {
		return Header{}
	}
	return h
}

// DestinationSSRC returns an array of SSRC values that this packet refers to.
func (r *RawPacket) DestinationSSRC() []uint32 {
	return []uint32{}
}
