// Portions of this file are:

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package webrtc

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"
)

// See https://golang.org/src/crypto/tls/generate_cert.go
func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}

// See https://golang.org/src/crypto/tls/generate_cert.go
func pemBlockForKey(priv interface{}) *pem.Block {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to marshal ECDSA private key: %v", err)
			os.Exit(2)
		}
		return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
	default:
		return nil
	}
}

// Generate a WebRTC-compatible certificate
//
// * Use elliptic curve digital signature algorithm (ECDSA) over the
//   P-256 curve.
// * Use a randomly generated serial number
// * Use "WebRTC" as the certificate's subject common name
// * Expire the certificate one month from now
// * Use ECDSA with SHA-256 as the signature algorithm (this is
//   different from the certificate fingerprint (i.e. a hash of the DER
//   ASN.1 encoding of the certificate), which must match the advertised
//   fingerprint in the SDP answer)
func generateCertificate() (certPEMBlock, keyPEMBlock []byte, fingerprint string, err error) {
	// Certificate will be valid for 30 days (Chrome default)
	notBefore := time.Now()
	notAfter := notBefore.Add(30 * 24 * time.Hour)

	// Generate a random serial number
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("failed to generate serial number: %s", err)
	}

	// Generate random elliptic curve key pair
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	// WebRTC certificate template
	template := x509.Certificate{
		SignatureAlgorithm: x509.ECDSAWithSHA256,
		SerialNumber:       serialNumber,
		Subject:            pkix.Name{CommonName: "WebRTC"},
		NotBefore:          notBefore,
		NotAfter:           notAfter,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(priv), priv)
	if err != nil {
		log.Fatalf("Failed to create certificate: %s", err)
	}

	certPEMBlock = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if certPEMBlock == nil {
		log.Fatalf("failed to write data to cert pem")
	}

	keyPEMBlock = pem.EncodeToMemory(pemBlockForKey(priv))
	if keyPEMBlock == nil {
		log.Fatalf("failed to write data to key pem")
	}

	h := sha256.Sum256(derBytes)
	fingerprint = fmt.Sprintf(
		"sha-256 "+
			"%x:%x:%x:%x:%x:%x:%x:%x:"+
			"%x:%x:%x:%x:%x:%x:%x:%x:"+
			"%x:%x:%x:%x:%x:%x:%x:%x:"+
			"%x:%x:%x:%x:%x:%x:%x:%x",
		h[0], h[1], h[2], h[3], h[4], h[5], h[6], h[7],
		h[8], h[9], h[10], h[11], h[12], h[13], h[14], h[15],
		h[16], h[17], h[18], h[19], h[20], h[21], h[22], h[23],
		h[24], h[25], h[26], h[27], h[28], h[29], h[30], h[31],
	)

	return certPEMBlock, keyPEMBlock, fingerprint, nil
}
