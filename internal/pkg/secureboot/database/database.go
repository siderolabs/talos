// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package database generates SecureBoot auto-enrollment database.
package database

import (
	"crypto/sha256"

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

// Generate generates a UEFI database to enroll the signing certificate.
//
// ref: https://blog.hansenpartnership.com/the-meaning-of-all-the-uefi-keys/
func Generate(enrolledCertificate []byte, signer pesign.CertificateSigner) ([]Entry, error) {
	// derive UUID from enrolled certificate
	uuid := uuid.NewHash(sha256.New(), uuid.NameSpaceX500, enrolledCertificate, 4)

	efiGUID := util.StringToGUID(uuid.String())

	// Create ESL
	db := signature.NewSignatureDatabase()
	if err := db.Append(signature.CERT_X509_GUID, *efiGUID, enrolledCertificate); err != nil {
		return nil, err
	}

	// Sign the ESL, but for each EFI variable
	_, signedDB, err := signature.SignEFIVariable(efivar.Db, db, signer.Signer(), signer.Certificate())
	if err != nil {
		return nil, err
	}

	_, signedKEK, err := signature.SignEFIVariable(efivar.KEK, db, signer.Signer(), signer.Certificate())
	if err != nil {
		return nil, err
	}

	_, signedPK, err := signature.SignEFIVariable(efivar.PK, db, signer.Signer(), signer.Certificate())
	if err != nil {
		return nil, err
	}

	return []Entry{
		{Name: constants.SignatureKeyAsset, Contents: signedDB.Bytes()},
		{Name: constants.KeyExchangeKeyAsset, Contents: signedKEK.Bytes()},
		{Name: constants.PlatformKeyAsset, Contents: signedPK.Bytes()},
	}, nil
}
