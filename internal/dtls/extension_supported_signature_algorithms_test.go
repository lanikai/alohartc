package dtls

import (
	"reflect"
	"testing"
)

func TestExtensionSupportedSignatureAlgorithms(t *testing.T) {

	rawExtensionSupportedSignatureAlgorithms := []byte{
		0x00, 0x0d,
		0x00, 0x12,
		0x00, 0x10,
		0x04, 0x01,
		0x04, 0x03,
		0x05, 0x01,
		0x05, 0x03,
		0x06, 0x01,
		0x06, 0x03,
		0x02, 0x01,
		0x02, 0x03,
	}
	parsedExtensionSupportedSignatureAlgorithms := &extensionSupportedSignatureAlgorithms{
		signatureHashAlgorithms: []signatureHashAlgorithm{
			signatureHashAlgorithm{HashAlgorithmSHA256, signatureAlgorithmRSA},
			signatureHashAlgorithm{HashAlgorithmSHA256, signatureAlgorithmECDSA},
			signatureHashAlgorithm{HashAlgorithmSHA384, signatureAlgorithmRSA},
			signatureHashAlgorithm{HashAlgorithmSHA384, signatureAlgorithmECDSA},
			signatureHashAlgorithm{HashAlgorithmSHA512, signatureAlgorithmRSA},
			signatureHashAlgorithm{HashAlgorithmSHA512, signatureAlgorithmECDSA},
			signatureHashAlgorithm{HashAlgorithmSHA1, signatureAlgorithmRSA},
			signatureHashAlgorithm{HashAlgorithmSHA1, signatureAlgorithmECDSA},
		},
	}

	raw, err := parsedExtensionSupportedSignatureAlgorithms.Marshal()
	if err != nil {
		t.Error(err)
	} else if !reflect.DeepEqual(raw, rawExtensionSupportedSignatureAlgorithms) {
		t.Errorf("extensionSupportedSignatureAlgorithms marshal: got %#v, want %#v", raw, rawExtensionSupportedSignatureAlgorithms)
	}
}
