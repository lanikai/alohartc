package main

import (
	"crypto/elliptic"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"log"
	"math/big"
	"os"
)

func main() {
	// Logging with line numbers
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Generate a  private key over secp256r1 (i.e. prime256k1) curve
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal(err)
	}

	// Save private key
	if keyBytes, err := x509.MarshalECPrivateKey(key); err != nil {
		log.Fatal(err)
	} else {
		pemEncoded := pem.EncodeToMemory(
			&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes},
		)

		if err = ioutil.WriteFile(
			"server-private.pem",
			pemEncoded,
			0644,
		); err != nil {
			log.Fatal(err)
		}
	}

	// Read template certificate (does not include private key)
	certBytes, err := ioutil.ReadFile("client.crt")
	if err != nil {
		log.Fatal(err)
	}

	cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		log.Fatal(err)
	}

	// Set random serial number
	cert.SerialNumber = new(big.Int)
	cert.SerialNumber.Exp(big.NewInt(2), big.NewInt(64), nil).Sub(cert.SerialNumber, big.NewInt(1))
	cert.SerialNumber, err = rand.Int(rand.Reader, cert.SerialNumber)

	// Create certificate
	derBytes, err := x509.CreateCertificate(
		rand.Reader,
		cert,
		cert,
		key.Public(),
		key,
	)
	if err != nil {
		log.Fatal(err)
	}

	certOut, err := os.Create("server.pem")
	if err != nil {
		log.Fatal(err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	ioutil.WriteFile("server.key", derBytes, 0600)

	// Compute fingerprint
	log.Printf("SHA-256 fingerprint: %x\n", sha256.Sum256(derBytes))
}
