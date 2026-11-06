// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cosign

import (
	"errors"
	"fmt"
	"time"

	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/sigstore/sigstore/pkg/signature"
)

// CheckOpts configures how image signatures are verified.
//
// It is created either with [NewKeylessCheckOpts] or with [NewPublicKeyCheckOpts].
type CheckOpts struct {
	// keyless verification: Fulcio certificate, Rekor transparency log entry, SCT
	trustedMaterial root.TrustedMaterial
	identity        verify.CertificateIdentity

	// public key verification: no transparency log, no timestamps
	publicKey signature.Verifier
}

// NewKeylessCheckOpts builds options for keyless verification.
//
// The signing certificate must chain to a Fulcio CA in the trusted material, carry an SCT,
// be logged in the transparency log, and match the identity (issuer and subject or subject regexp).
func NewKeylessCheckOpts(trustedMaterial root.TrustedMaterial, issuer, subject, subjectRegExp string) (CheckOpts, error) {
	if trustedMaterial == nil {
		return CheckOpts{}, errors.New("trusted material is required for keyless verification")
	}

	sanMatcher, err := verify.NewSANMatcher(subject, subjectRegExp)
	if err != nil {
		return CheckOpts{}, fmt.Errorf("invalid subject matcher: %w", err)
	}

	issuerMatcher, err := verify.NewIssuerMatcher(issuer, "")
	if err != nil {
		return CheckOpts{}, fmt.Errorf("invalid issuer matcher: %w", err)
	}

	identity, err := verify.NewCertificateIdentity(sanMatcher, issuerMatcher, certificate.Extensions{})
	if err != nil {
		return CheckOpts{}, fmt.Errorf("invalid certificate identity: %w", err)
	}

	return CheckOpts{
		trustedMaterial: trustedMaterial,
		identity:        identity,
	}, nil
}

// NewPublicKeyCheckOpts builds options for verification with a static public key.
//
// Neither the transparency log nor timestamps are checked, so verification works offline.
func NewPublicKeyCheckOpts(publicKey signature.Verifier) CheckOpts {
	return CheckOpts{
		publicKey: publicKey,
	}
}

func (co CheckOpts) keyless() bool {
	return co.publicKey == nil
}

// verify verifies the signed entity against the artifact policy.
//
// The options mirror what cosign uses for the same verification modes.
func (co CheckOpts) verify(entity verify.SignedEntity, artifactPolicyOption verify.ArtifactPolicyOption) error {
	var (
		trustedMaterial root.TrustedMaterial
		verifierOptions []verify.VerifierOption
		policyOptions   []verify.PolicyOption
	)

	if co.keyless() {
		trustedMaterial = co.trustedMaterial

		verifierOptions = []verify.VerifierOption{
			verify.WithSignedCertificateTimestamps(1),
			verify.WithTransparencyLog(1),
		}

		rekorV2Only, err := hasOnlyRekorV2Entries(entity)
		if err != nil {
			return err
		}

		if rekorV2Only {
			// Rekor v2 entries carry no integrated time, so the certificate validity
			// has to be checked against a signed timestamp.
			verifierOptions = append(verifierOptions, verify.WithSignedTimestamps(1))
		} else {
			verifierOptions = append(verifierOptions, verify.WithIntegratedTimestamps(1))
		}

		policyOptions = []verify.PolicyOption{verify.WithCertificateIdentity(co.identity)}
	} else {
		key := root.NewExpiringKey(co.publicKey, time.Time{}, time.Time{})

		trustedMaterial = root.NewTrustedPublicKeyMaterial(func(string) (root.TimeConstrainedVerifier, error) {
			return key, nil
		})

		verifierOptions = []verify.VerifierOption{verify.WithNoObserverTimestamps()}
		policyOptions = []verify.PolicyOption{verify.WithKey()}
	}

	verifier, err := verify.NewVerifier(trustedMaterial, verifierOptions...)
	if err != nil {
		return fmt.Errorf("failed to create verifier: %w", err)
	}

	if _, err = verifier.Verify(entity, verify.NewPolicy(artifactPolicyOption, policyOptions...)); err != nil {
		return err
	}

	return nil
}

// hasOnlyRekorV2Entries reports whether all transparency log entries come from Rekor v2.
//
// Rekor v2 entries have no integrated time. A mix of v1 and v2 entries is treated as v1.
func hasOnlyRekorV2Entries(entity verify.SignedEntity) (bool, error) {
	entries, err := entity.TlogEntries()
	if err != nil {
		return false, fmt.Errorf("failed to get transparency log entries: %w", err)
	}

	var hasV1, hasV2 bool

	for _, entry := range entries {
		if entry.IntegratedTime().IsZero() {
			hasV2 = true
		} else {
			hasV1 = true
		}
	}

	return hasV2 && !hasV1, nil
}
