package main

import (
	"crypto/elliptic"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"io/ioutil"
	"log"
	"math/big"
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
		if err = ioutil.WriteFile(
			"private.der",
			keyBytes,
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

	if err := ioutil.WriteFile("client_out.crt", derBytes, 0644); err != nil {
		log.Fatal(err)
	}
}
