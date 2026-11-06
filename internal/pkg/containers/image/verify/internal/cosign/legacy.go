// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cosign

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/containerd/containerd/v2/core/remotes"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	rekorpb "github.com/sigstore/protobuf-specs/gen/pb-go/rekor/v1"
	sgbundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/tlog"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/sigstore/sigstore/pkg/signature/payload"
)

// Legacy cosign signature layer annotations.
const (
	signatureAnnotationKey   = "dev.cosignproject.cosign/signature"
	certificateAnnotationKey = "dev.sigstore.cosign/certificate"
	bundleAnnotationKey      = "dev.sigstore.cosign/bundle"
)

// legacySignature is a legacy cosign signature (a .sig tag layer) presented as a sigstore-go signed entity.
//
// A legacy signature with a Rekor bundle annotation carries the same material as a v0.1 sigstore bundle:
// a message signature over the simple signing payload, the signing certificate, and a transparency log
// entry with an inclusion promise (signed entry timestamp). sigstore-go verifies it the same way.
type legacySignature struct {
	verify.BaseSignedEntity

	verificationContent verify.VerificationContent
	signatureContent    *sgbundle.MessageSignature
	tlogEntries         []*tlog.Entry

	payload []byte
}

func (s *legacySignature) HasInclusionPromise() bool {
	return len(s.tlogEntries) > 0
}

func (s *legacySignature) VerificationContent() (verify.VerificationContent, error) {
	return s.verificationContent, nil
}

func (s *legacySignature) SignatureContent() (verify.SignatureContent, error) {
	return s.signatureContent, nil
}

func (s *legacySignature) TlogEntries() ([]*tlog.Entry, error) {
	return s.tlogEntries, nil
}

func (s *legacySignature) Timestamps() ([][]byte, error) {
	return nil, nil
}

func (s *legacySignature) Version() (string, error) {
	return "v0.1", nil
}

// rekorBundle is the Rekor bundle stored in the legacy signature layer annotation.
type rekorBundle struct {
	SignedEntryTimestamp []byte `json:"SignedEntryTimestamp"`
	Payload              struct {
		Body           string `json:"body"`
		IntegratedTime int64  `json:"integratedTime"`
		LogIndex       int64  `json:"logIndex"`
		LogID          string `json:"logID"`
	} `json:"Payload"`
}

func (b *rekorBundle) tlogEntry() (*tlog.Entry, error) {
	body, err := base64.StdEncoding.DecodeString(b.Payload.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Rekor bundle body: %w", err)
	}

	logID, err := hex.DecodeString(b.Payload.LogID)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Rekor bundle log ID: %w", err)
	}

	var kindVersion struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
	}

	if err = json.Unmarshal(body, &kindVersion); err != nil {
		return nil, fmt.Errorf("failed to parse Rekor bundle body: %w", err)
	}

	return tlog.ParseTransparencyLogEntry(&rekorpb.TransparencyLogEntry{
		LogIndex: b.Payload.LogIndex,
		LogId: &protocommon.LogId{
			KeyId: logID,
		},
		KindVersion: &rekorpb.KindVersion{
			Kind:    kindVersion.Kind,
			Version: kindVersion.APIVersion,
		},
		IntegratedTime: b.Payload.IntegratedTime,
		InclusionPromise: &rekorpb.InclusionPromise{
			SignedEntryTimestamp: b.SignedEntryTimestamp,
		},
		CanonicalizedBody: body,
	})
}

// buildLegacySignature constructs a signed entity from a cosign legacy signature layer.
//
// The layer content is the simple signing payload (the data that was signed). The cryptographic
// signature, the certificate and the Rekor bundle are read from the layer annotations.
//
// For public key verification, the certificate and the Rekor bundle are ignored: neither the
// transparency log nor timestamps are checked.
func buildLegacySignature(ctx context.Context, fetcher remotes.Fetcher, layer ocispec.Descriptor, co CheckOpts) (*legacySignature, error) {
	b64sig, ok := layer.Annotations[signatureAnnotationKey]
	if !ok {
		return nil, errors.New("missing signature annotation")
	}

	rawSig, err := base64.StdEncoding.DecodeString(b64sig)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature annotation: %w", err)
	}

	// Fetch the layer content: the simple signing payload (what was signed).
	rc, err := fetcher.Fetch(ctx, layer)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch signature layer: %w", err)
	}

	defer rc.Close() //nolint:errcheck

	signedPayload, err := io.ReadAll(io.LimitReader(rc, maxPayloadSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read signature layer: %w", err)
	}

	payloadDigest := sha256.Sum256(signedPayload)

	sig := &legacySignature{
		signatureContent: sgbundle.NewMessageSignature(payloadDigest[:], "sha256", rawSig),
		payload:          signedPayload,
	}

	if !co.keyless() {
		sig.verificationContent = &sgbundle.PublicKey{}

		return sig, nil
	}

	sig.verificationContent, sig.tlogEntries, err = legacyKeylessMaterial(layer.Annotations)
	if err != nil {
		return nil, err
	}

	return sig, nil
}

// legacyKeylessMaterial reads the signing certificate and the transparency log entry from the layer annotations.
func legacyKeylessMaterial(annotations map[string]string) (verify.VerificationContent, []*tlog.Entry, error) {
	certPEM, ok := annotations[certificateAnnotationKey]
	if !ok {
		return nil, nil, errors.New("missing certificate annotation")
	}

	certs, err := cryptoutils.UnmarshalCertificatesFromPEM([]byte(certPEM))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse certificate annotation: %w", err)
	}

	if len(certs) == 0 {
		return nil, nil, errors.New("no certificate in certificate annotation")
	}

	bundleJSON, ok := annotations[bundleAnnotationKey]
	if !ok {
		return nil, nil, errors.New("missing Rekor bundle annotation")
	}

	var rb rekorBundle

	if err = json.Unmarshal([]byte(bundleJSON), &rb); err != nil {
		return nil, nil, fmt.Errorf("failed to parse Rekor bundle annotation: %w", err)
	}

	entry, err := rb.tlogEntry()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert Rekor bundle: %w", err)
	}

	// Intermediate certificates come from the trusted root, so the chain annotation is not used.
	return sgbundle.NewCertificate(certs[0]), []*tlog.Entry{entry}, nil
}

// verifyClaim checks that the (verified) simple signing payload references the image being verified.
func verifyClaim(signedPayload []byte, imageDigest digest.Digest) error {
	var claim payload.SimpleContainerImage

	if err := json.Unmarshal(signedPayload, &claim); err != nil {
		return fmt.Errorf("failed to parse signed payload: %w", err)
	}

	if foundDigest := claim.Critical.Image.DockerManifestDigest; foundDigest != imageDigest.String() {
		return fmt.Errorf("invalid or missing digest in claim: %s", foundDigest)
	}

	return nil
}
