package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
)

func main() {
	// Create keys directory
	os.MkdirAll("keys", 0700)

	// Generate 2048-bit RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	// Save private key
	privFile, _ := os.Create("keys/private.pem")
	pem.Encode(privFile, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	privFile.Close()

	// Save public key
	pubBytes, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	pubFile, _ := os.Create("keys/public.pem")
	pem.Encode(pubFile, &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})
	pubFile.Close()

	fmt.Println("Keys generated successfully:")
	fmt.Println("  keys/private.pem")
	fmt.Println("  keys/public.pem")
}
