// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package azure

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azcertificates"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/internal/pkg/secureboot/pesign"
)

// SecureBootSigner implements pesign.CertificateSigner interface.
type SecureBootSigner struct {
	keySigner *KeySigner
	cert      *x509.Certificate
}

// Verify interface.
var _ pesign.CertificateSigner = (*SecureBootSigner)(nil)

// Signer returns the signer.
func (s *SecureBootSigner) Signer() crypto.Signer {
	return s.keySigner
}

// Certificate returns the certificate.
func (s *SecureBootSigner) Certificate() *x509.Certificate {
	return s.cert
}

// NewSecureBootSigner creates a new SecureBootSigner.
//
//nolint:gocyclo
func NewSecureBootSigner(ctx context.Context, vaultURL, certificateID, certificateVersion string) (*SecureBootSigner, error) {
	certsClient, err := getCertsClient(vaultURL)
	if err != nil {
		return nil, fmt.Errorf("failed to build Azure certificates client: %w", err)
	}

	resp, err := certsClient.GetCertificate(ctx, certificateID, certificateVersion, &azcertificates.GetCertificateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get certificate: %w", err)
	}

	// download the certificate from secrets storage by secret ID
	secretsClient, err := getSecretsClient(vaultURL)
	if err != nil {
		return nil, fmt.Errorf("failed to build Azure secrets client: %w", err)
	}

	SID := pointer.SafeDeref(resp.SID)

	secretsResp, err := secretsClient.GetSecret(ctx, SID.Name(), SID.Version(), &azsecrets.GetSecretOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch certificate secret: %w", err)
	}

	certData := []byte(pointer.SafeDeref(secretsResp.Value))

	var cert *x509.Certificate

	for {
		var certBlock *pem.Block

		certBlock, certData = pem.Decode(certData)
		if certBlock == nil {
			break
		}

		if certBlock.Type == "CERTIFICATE" {
			cert, err = x509.ParseCertificate(certBlock.Bytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse certificate: %w", err)
			}

			break
		}
	}

	if cert == nil {
		return nil, fmt.Errorf("failed to decode certificate")
	}

	// initialize key signer via existing implementation
	KID := pointer.SafeDeref(resp.KID)

	keySigner, err := NewPCRSigner(ctx, vaultURL, KID.Name(), KID.Version())
	if err != nil {
		return nil, fmt.Errorf("failed to initialize certificate key signer: %w", err)
	}

	return &SecureBootSigner{
		cert:      cert,
		keySigner: keySigner,
	}, nil
}
