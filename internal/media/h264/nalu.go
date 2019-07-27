package h264

type NALU []byte

func (nalu NALU) ForbiddenBit() byte {
	return nalu[0] & 0x80 >> 7
}

func (nalu NALU) NRI() byte {
	return nalu[0] & 0x60 >> 5
}

func (nalu NALU) Type() byte {
	return nalu[0] & 0x1f
}
