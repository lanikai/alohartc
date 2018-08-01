package main

import (
	"encoding/pem"
)

func main() {
	x509Encoded, _ := x509.MarshalECPrivateKey(privateKey)
	pemEncoded := pem.EncodeToMemory(
		&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded},
	)
}
