// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package database generates SecureBoot auto-enrollment database.
package database

import (
	"crypto/sha256"
	"crypto/x509"
	"embed"
	"path/filepath"
	"sync"

	"github.com/foxboron/go-uefi/efi/signature"
	"github.com/foxboron/go-uefi/efi/util"
	"github.com/foxboron/go-uefi/efivar"
	"github.com/google/uuid"

	"github.com/siderolabs/talos/internal/pkg/secureboot/pesign"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Entry is a UEFI database entry.
type Entry struct {
	Name     string
	Contents []byte
}

const (
	microsoftSignatureOwnerGUID = "77fa9abd-0359-4d32-bd60-28f4e78f784b"
)

// Well-known UEFI DB certificates (DER data).
//
//go:embed certs/db/*.der
var wellKnownDB embed.FS

// Well-known UEFI KEK certificates (PEM data).
//
//go:embed certs/kek/*.der
var wellKnownKEK embed.FS

func loadWellKnownCertificates(fs embed.FS, path string) ([]*x509.Certificate, error) {
	certs := []*x509.Certificate{}

	files, err := fs.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		data, err := fs.ReadFile(filepath.Join(path, file.Name()))
		if err != nil {
			return nil, err
		}

		cert, err := x509.ParseCertificate(data)
		if err != nil {
			return nil, err
		}

		certs = append(certs, cert)
	}

	return certs, nil
}

var wellKnownDBCertificates = sync.OnceValue(func() []*x509.Certificate {
	certs, err := loadWellKnownCertificates(wellKnownDB, "certs/db")
	if err != nil {
		panic(err)
	}

	return certs
})

var wellKnownKEKCertificates = sync.OnceValue(func() []*x509.Certificate {
	certs, err := loadWellKnownCertificates(wellKnownKEK, "certs/kek")
	if err != nil {
		panic(err)
	}

	return certs
})

// Options for Generate.
type Options struct {
	IncludeWellKnownCertificates bool
}

// Option is a functional option for Generate.
type Option func(*Options)

// IncludeWellKnownCertificates is an option to include well-known certificates.
func IncludeWellKnownCertificates(v bool) Option {
	return func(o *Options) {
		o.IncludeWellKnownCertificates = v
	}
}

// Generate generates a UEFI database to enroll the signing certificate.
//
// ref: https://blog.hansenpartnership.com/the-meaning-of-all-the-uefi-keys/
//
//nolint:gocyclo
func Generate(enrolledCertificate []byte, signer pesign.CertificateSigner, opts ...Option) ([]Entry, error) {
	var options Options

	for _, opt := range opts {
		opt(&options)
	}

	// derive UUID from enrolled certificate
	uuid := uuid.NewHash(sha256.New(), uuid.NameSpaceX500, enrolledCertificate, 4)

	efiGUID := util.StringToGUID(uuid.String())

	// Create PK ESL
	pk := signature.NewSignatureDatabase()
	if err := pk.Append(signature.CERT_X509_GUID, *efiGUID, enrolledCertificate); err != nil {
		return nil, err
	}

	_, signedPK, err := signature.SignEFIVariable(efivar.PK, pk, signer.Signer(), signer.Certificate())
	if err != nil {
		return nil, err
	}

	// Create KEK ESL
	kek := signature.NewSignatureDatabase()
	if err := kek.Append(signature.CERT_X509_GUID, *efiGUID, enrolledCertificate); err != nil {
		return nil, err
	}

	if options.IncludeWellKnownCertificates {
		owner := util.StringToGUID(microsoftSignatureOwnerGUID)
		for _, cert := range wellKnownKEKCertificates() {
			if err := kek.Append(signature.CERT_X509_GUID, *owner, cert.Raw); err != nil {
				return nil, err
			}
		}
	}

	_, signedKEK, err := signature.SignEFIVariable(efivar.KEK, kek, signer.Signer(), signer.Certificate())
	if err != nil {
		return nil, err
	}

	// Create db ESL
	db := signature.NewSignatureDatabase()
	if err := db.Append(signature.CERT_X509_GUID, *efiGUID, enrolledCertificate); err != nil {
		return nil, err
	}

	if options.IncludeWellKnownCertificates {
		owner := util.StringToGUID(microsoftSignatureOwnerGUID)
		for _, cert := range wellKnownDBCertificates() {
			if err := db.Append(signature.CERT_X509_GUID, *owner, cert.Raw); err != nil {
				return nil, err
			}
		}
	}

	_, signedDB, err := signature.SignEFIVariable(efivar.Db, db, signer.Signer(), signer.Certificate())
	if err != nil {
		return nil, err
	}

	return []Entry{
		{Name: constants.SignatureKeyAsset, Contents: signedDB.Bytes()},
		{Name: constants.KeyExchangeKeyAsset, Contents: signedKEK.Bytes()},
		{Name: constants.PlatformKeyAsset, Contents: signedPK.Bytes()},
	}, nil
}
