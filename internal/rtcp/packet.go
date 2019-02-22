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

// Packet represents an RTCP packet, a protocol used for out-of-band statistics and control information for an RTP session
type Packet interface {
	Header() Header
	// DestinationSSRC returns an array of SSRC values that this packet refers to.
	DestinationSSRC() []uint32

	Marshal() ([]byte, error)
	Unmarshal(rawPacket []byte) error
}

// Unmarshal is a factory a polymorphic RTCP packet, and its header,
func Unmarshal(rawPacket []byte) (Packet, Header, error) {
	var h Header
	var p Packet

	err := h.Unmarshal(rawPacket)
	if err != nil {
		return nil, h, err
	}

	switch h.Type {
	//	case TypeSenderReport:
	//		p = new(SenderReport)

	case TypeReceiverReport:
		p = new(ReceiverReport)

		//	case TypeSourceDescription:
		//		p = new(SourceDescription)
		//
		//	case TypeGoodbye:
		//		p = new(Goodbye)
		//
		//	case TypeTransportSpecificFeedback:
		//		switch h.Count {
		//		case FormatTLN:
		//			p = new(TransportLayerNack)
		//		case FormatRRR:
		//			p = new(RapidResynchronizationRequest)
		//		default:
		//			p = new(RawPacket)
		//		}
		//
		//	case TypePayloadSpecificFeedback:
		//		switch h.Count {
		//		case FormatPLI:
		//			p = new(PictureLossIndication)
		//		case FormatSLI:
		//			p = new(SliceLossIndication)
		//		default:
		//			p = new(RawPacket)
		//		}

	default:
		p = new(RawPacket)
	}

	err = p.Unmarshal(rawPacket)
	return p, h, err
}
