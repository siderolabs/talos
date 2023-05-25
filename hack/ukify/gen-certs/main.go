// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// gen-certs is a tool to generate UKI signing keys and certificates.
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/siderolabs/crypto/x509"
)

func generateSigningCerts(path, prefix, commonName string) error {
	currentTime := time.Now()

	opts := []x509.Option{
		x509.RSA(true),
		x509.CommonName(commonName),
		x509.NotAfter(currentTime.Add(24 * time.Hour)),
		x509.NotBefore(currentTime),
	}

	signingKey, err := x509.NewSelfSignedCertificateAuthority(opts...)
	if err != nil {
		return err
	}

	if err = os.WriteFile(filepath.Join(path, prefix+"-signing-cert.pem"), signingKey.CrtPEM, 0o600); err != nil {
		return err
	}

	if err = os.WriteFile(filepath.Join(path, prefix+"-signing-key.pem"), signingKey.KeyPEM, 0o600); err != nil {
		return err
	}

	pemKey := x509.PEMEncodedKey{
		Key: signingKey.KeyPEM,
	}

	privKey, err := pemKey.GetRSAKey()
	if err != nil {
		return err
	}

	if err = os.WriteFile(filepath.Join(path, prefix+"-signing-public-key.pem"), privKey.PublicKeyPEM, 0o600); err != nil {
		return err
	}

	return nil
}

func run() error {
	var outputPath string

	flag.StringVar(&outputPath, "output-path", "_out", "path to output directory")
	flag.Parse()

	if err := os.MkdirAll(outputPath, 0o755); err != nil {
		return err
	}

	if err := generateSigningCerts(outputPath, "uki", "Test UKI Signing Key"); err != nil {
		return err
	}

	if err := generateSigningCerts(outputPath, "pcr", "Test PCR Signing Key"); err != nil {
		return err
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
