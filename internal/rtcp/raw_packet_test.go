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

import (
	"reflect"
	"testing"
)

func TestRawPacketRoundTrip(t *testing.T) {
	for _, test := range []struct {
		Name               string
		Packet             RawPacket
		WantMarshalError   error
		WantUnmarshalError error
	}{
		{
			Name: "valid",
			Packet: RawPacket([]byte{
				// v=2, p=0, count=1, BYE, len=12
				0x81, 0xcb, 0x00, 0x0c,
				// ssrc=0x902f9e2e
				0x90, 0x2f, 0x9e, 0x2e,
				// len=3, text=FOO
				0x03, 0x46, 0x4f, 0x4f,
			}),
		},
		{
			Name:               "short header",
			Packet:             RawPacket([]byte{0x00}),
			WantUnmarshalError: errPacketTooShort,
		},
		{
			Name: "invalid header",
			Packet: RawPacket([]byte{
				// v=0, p=0, count=0, RR, len=4
				0x00, 0xc9, 0x00, 0x04,
			}),
			WantUnmarshalError: errBadVersion,
		},
	} {
		data, err := test.Packet.Marshal()
		if got, want := err, test.WantMarshalError; got != want {
			t.Fatalf("Marshal %q: err = %v, want %v", test.Name, got, want)
		}
		if err != nil {
			continue
		}

		var decoded RawPacket
		err = decoded.Unmarshal(data)
		if got, want := err, test.WantUnmarshalError; got != want {
			t.Fatalf("Unmarshal %q: err = %v, want %v", test.Name, got, want)
		}
		if err != nil {
			continue
		}

		if got, want := decoded, test.Packet; !reflect.DeepEqual(got, want) {
			t.Fatalf("%q raw round trip: got %#v, want %#v", test.Name, got, want)
		}
	}
}
