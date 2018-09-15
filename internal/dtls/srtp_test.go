package dtls

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestKDF(t *testing.T) {
	testMasterKey, _ := hex.DecodeString("E1F97A0D3E018BE0D64FA32C06DE4139")
	testMasterSalt, _ := hex.DecodeString("0EC675AD498AFEEBB6960B3AABE6")
	testPacketIndex := uint(0)
	testKeyDerivationRate := uint(0)

	expectedSRTPKey, _ := hex.DecodeString("C61E7A93744F39EE10734AFE3FF7A087")
	expectedSRTPSalt, _ := hex.DecodeString("30CBBC08863D8C85D49DB34A9AE1")
	expectedSRTCPKey, _ := hex.DecodeString("4C1AA45A81F73D61C800BBB00FBB1EAA")
	expectedSRTCPSalt, _ := hex.DecodeString("9581C7AD87B3E530BF3E4454A8B3")

	srtpKey, srtpSalt, srtcpKey, srtcpSalt, err := kdf(testMasterKey, testMasterSalt, testPacketIndex, testKeyDerivationRate, 16, 14)
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(srtpKey, expectedSRTPKey) {
		t.Errorf("srtp cipher key is incorrect.\nreceived: %x\nexpected: %x\n", srtpKey, expectedSRTPKey)
	}

	if !bytes.Equal(srtpSalt, expectedSRTPSalt) {
		t.Errorf("srtp salt key is incorrect.\nreceived: %x\nexpected: %x\n", srtpSalt, expectedSRTPSalt)
	}

	if !bytes.Equal(srtcpKey, expectedSRTCPKey) {
		t.Errorf("srtcp cipher key is incorrect.\nreceived: %x\nexpected: %x\n", srtcpKey, expectedSRTCPKey)
	}

	if !bytes.Equal(srtcpSalt, expectedSRTCPSalt) {
		t.Errorf("srtcp salt key is incorrect.\nreceived: %x\nexpected: %x\n", srtcpSalt, expectedSRTCPSalt)
	}
}
