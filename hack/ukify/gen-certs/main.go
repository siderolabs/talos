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

	"github.com/foxboron/go-uefi/efi"
	"github.com/foxboron/go-uefi/efi/signature"
	"github.com/foxboron/go-uefi/efi/util"
	"github.com/google/uuid"

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

// ref: https://blog.hansenpartnership.com/the-meaning-of-all-the-uefi-keys/
func generateSecureBootFiles(path, prefix string) error {
	uuid, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	efiGUID := util.StringToGUID(uuid.String())

	// Reuse the generated test signing key for secure boot
	pem, err := x509.NewCertificateAndKeyFromFiles(filepath.Join(path, prefix+"-signing-cert.pem"), filepath.Join(path, prefix+"-signing-key.pem"))
	if err != nil {
		return err
	}
	cert, err := pem.GetCert()
	if err != nil {
		return err
	}
	key, err := pem.GetRSAKey()
	if err != nil {
		return err
	}

	// Create ESL
	db := signature.NewSignatureDatabase()
	if err = db.Append(signature.CERT_X509_GUID, *efiGUID, pem.Crt); err != nil {
		return err
	}

	// Sign the ESL, but for each EFI variable
	signedDb, err := efi.SignEFIVariable(key, cert, "db", db.Bytes())
	if err != nil {
		return err
	}

	signedKEK, err := efi.SignEFIVariable(key, cert, "KEK", db.Bytes())
	if err != nil {
		return err
	}

	signedPK, err := efi.SignEFIVariable(key, cert, "PK", db.Bytes())
	if err != nil {
		return err
	}

	// Output all files with sd-boot convential names for auto-enrolment
	if err = os.WriteFile(filepath.Join(path, "db.auth"), signedDb, 0o600); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(path, "KEK.auth"), signedKEK, 0o600); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(path, "PK.auth"), signedPK, 0o600); err != nil {
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

	if err := generateSecureBootFiles(outputPath, "uki"); err != nil {
		return err
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
