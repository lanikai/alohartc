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

func TestHeaderUnmarshal(t *testing.T) {
	for _, test := range []struct {
		Name      string
		Data      []byte
		Want      Header
		WantError error
	}{
		{
			Name: "valid",
			Data: []byte{
				// v=2, p=0, count=1, RR, len=7
				0x81, 0xc9, 0x00, 0x07,
			},
			Want: Header{
				Padding: false,
				Count:   1,
				Type:    TypeReceiverReport,
				Length:  7,
			},
		},
		{
			Name: "also valid",
			Data: []byte{
				// v=2, p=1, count=1, BYE, len=7
				0xa1, 0xcc, 0x00, 0x07,
			},
			Want: Header{
				Padding: true,
				Count:   1,
				Type:    TypeApplicationDefined,
				Length:  7,
			},
		},
		{
			Name: "bad version",
			Data: []byte{
				// v=0, p=0, count=0, RR, len=4
				0x00, 0xc9, 0x00, 0x04,
			},
			WantError: errBadVersion,
		},
	} {
		var h Header
		err := h.Unmarshal(test.Data)
		if got, want := err, test.WantError; got != want {
			t.Fatalf("Unmarshal %q header: err = %v, want %v", test.Name, got, want)
		}
		if err != nil {
			continue
		}

		if got, want := h, test.Want; !reflect.DeepEqual(got, want) {
			t.Fatalf("Unmarshal %q header: got %v, want %v", test.Name, got, want)
		}
	}
}
func TestHeaderRoundTrip(t *testing.T) {
	for _, test := range []struct {
		Name      string
		Header    Header
		WantError error
	}{
		{
			Name: "valid",
			Header: Header{
				Padding: true,
				Count:   31,
				Type:    TypeSenderReport,
				Length:  4,
			},
		},
		{
			Name: "also valid",
			Header: Header{
				Padding: false,
				Count:   28,
				Type:    TypeReceiverReport,
				Length:  65535,
			},
		},
		{
			Name: "invalid count",
			Header: Header{
				Count: 40,
			},
			WantError: errInvalidHeader,
		},
	} {
		data, err := test.Header.Marshal()
		if got, want := err, test.WantError; got != want {
			t.Errorf("Marshal %q: err = %v, want %v", test.Name, got, want)
		}
		if err != nil {
			continue
		}

		var decoded Header
		if err := decoded.Unmarshal(data); err != nil {
			t.Errorf("Unmarshal %q: %v", test.Name, err)
		}

		if got, want := decoded, test.Header; !reflect.DeepEqual(got, want) {
			t.Errorf("%q header round trip: got %#v, want %#v", test.Name, got, want)
		}
	}
}
