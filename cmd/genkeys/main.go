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
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Printf("Error generating private key: %v\n", err)
		os.Exit(1)
	}

	// Save private key
	privateKeyFile, err := os.Create("test_private.pem")
	if err != nil {
		fmt.Printf("Error creating private key file: %v\n", err)
		os.Exit(1)
	}
	defer privateKeyFile.Close()

	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		fmt.Printf("Error encoding private key: %v\n", err)
		os.Exit(1)
	}

	// Save public key
	publicKeyFile, err := os.Create("test_public.pem")
	if err != nil {
		fmt.Printf("Error creating public key file: %v\n", err)
		os.Exit(1)
	}
	defer publicKeyFile.Close()

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		fmt.Printf("Error marshaling public key: %v\n", err)
		os.Exit(1)
	}

	publicKeyPEM := &pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: publicKeyBytes,
	}

	if err := pem.Encode(publicKeyFile, publicKeyPEM); err != nil {
		fmt.Printf("Error encoding public key: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Test keys generated successfully:")
	fmt.Println("   Private key: test_private.pem")
	fmt.Println("   Public key:  test_public.pem")
	fmt.Println("")
	fmt.Println("📋 Add these keys to your config.json:")
	fmt.Println("")
	fmt.Println(`{
  "auth": {
    "enabled": true,
    "private_key": "-----BEGIN RSA PRIVATE KEY-----...",`)
	fmt.Println(`    "public_key": "-----BEGIN RSA PUBLIC KEY-----..."`)
	fmt.Println(`  }
}`)
}
