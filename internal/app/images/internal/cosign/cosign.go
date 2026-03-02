// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cosign provides cosign-based image signature verification via Talos pull process.
package cosign

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/cosign/v3/pkg/cosign/bundle"
	"github.com/sigstore/cosign/v3/pkg/oci"
	"github.com/sigstore/cosign/v3/pkg/oci/static"
	sgbundle "github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"go.uber.org/zap"
)

const (
	maxManifestSize = 128 * 1024
	maxBundleSize   = 1024 * 1024
	maxPayloadSize  = 1024 * 1024
	maxLayers       = 100

	sigstoreBundleMediaTypePrefix = "application/vnd.dev.sigstore.bundle"
	sigstoreBundleV03ArtifactType = "application/vnd.dev.sigstore.bundle.v0.3+json"
)

// VerifyResult represents the result of a signature verification attempt.
type VerifyResult struct {
	Message string
}

// VerifyImage verifies the given image reference and digest against the provided verification configuration.
//
// It checks in priority order:
//  1. OCI referrers API for new-style sigstore bundles (sha256:xxx referrers).
//  2. The bundle tag (sha256-xxx, no .sig suffix) for new-style sigstore bundles stored as a tag.
//  3. The legacy .sig tag (sha256-xxx.sig) for legacy cosign signature layers.
//
// All registry I/O goes through the provided resolver, which handles authentication and mirror routing.
//
// The verifiers are in opts, if any of the verifiers returns true for bundle verification, the image is considered verified.
func VerifyImage(ctx context.Context, logger *zap.Logger, resolver remotes.Resolver, imageRef name.Digest, co cosign.CheckOpts) (*VerifyResult, error) {
	logger = logger.With(zap.Stringer("image", imageRef))

	imageDigest, err := v1.NewHash(imageRef.DigestStr())
	if err != nil {
		return nil, fmt.Errorf("failed to parse image digest: %w", err)
	}

	digestBytes, err := hex.DecodeString(imageDigest.Hex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode digest hex: %w", err)
	}

	artifactPolicyOption := verify.WithArtifactDigest(imageDigest.Algorithm, digestBytes)

	// Step 1: try OCI referrers for new-style sigstore bundles.
	fetcher, err := resolver.Fetcher(ctx, imageRef.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to get fetcher: %w", err)
	}

	if refFetcher, ok := fetcher.(remotes.ReferrersFetcher); ok {
		referrers, err := refFetcher.FetchReferrers(ctx, digest.Digest(imageRef.DigestStr()),
			remotes.WithReferrerArtifactTypes(sigstoreBundleV03ArtifactType))
		if err == nil {
			bundleRefs := filterBundleReferrers(referrers)
			if len(bundleRefs) > 0 {
				logger.Debug("verifying via OCI referrer bundles", zap.Int("count", len(bundleRefs)))

				return verifyBundleReferrers(ctx, logger, fetcher, bundleRefs, artifactPolicyOption, co)
			}
		}
	}

	// Step 2: try bundle tag (sha256-xxx) for new-style sigstore bundles stored as a tag.
	logger.Debug("no bundle referrers found, trying bundle tag")

	if verified, result, err := verifyFromBundleTag(ctx, logger, resolver, imageRef, artifactPolicyOption, co); verified {
		return result, err
	}

	// Step 3: fall back to the legacy .sig tag (sha256-xxx.sig).
	logger.Debug("no bundle tag found, falling back to legacy .sig tag")

	return verifyFromLegacySigTag(ctx, logger, resolver, imageRef, imageDigest, co)
}

// filterBundleReferrers returns only referrers whose ArtifactType indicates a sigstore bundle.
func filterBundleReferrers(referrers []ocispec.Descriptor) []ocispec.Descriptor {
	var out []ocispec.Descriptor

	for _, r := range referrers {
		if strings.HasPrefix(r.ArtifactType, sigstoreBundleMediaTypePrefix) {
			out = append(out, r)
		}
	}

	return out
}

// verifyBundleReferrers verifies a set of OCI referrer descriptors as new-style sigstore bundles.
//
// Each referrer points to an OCI image manifest whose single layer contains the sigstore bundle JSON.
func verifyBundleReferrers(
	ctx context.Context, logger *zap.Logger, fetcher remotes.Fetcher, referrers []ocispec.Descriptor, artifactPolicyOption verify.ArtifactPolicyOption, co cosign.CheckOpts,
) (*VerifyResult, error) {
	var lastErr error

	for _, referrer := range referrers {
		b, err := fetchBundleFromReferrer(ctx, fetcher, referrer)
		if err != nil {
			logger.Debug("failed to fetch bundle referrer", zap.String("digest", referrer.Digest.String()), zap.Error(err))
			lastErr = err

			continue
		}

		co.NewBundleFormat = true

		if _, err := cosign.VerifyNewBundle(ctx, &co, artifactPolicyOption, b); err != nil {
			logger.Debug("bundle referrer verification failed", zap.String("digest", referrer.Digest.String()), zap.Error(err))
			lastErr = err

			continue
		}

		logger.Debug("bundle referrer verified", zap.String("digest", referrer.Digest.String()))

		return &VerifyResult{
			Message: fmt.Sprintf("verified via bundle referrer with digest %s", referrer.Digest.String()),
		}, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("no valid bundle referrer: %w", lastErr)
	}

	return nil, errors.New("no valid bundle referrers")
}

// verifyFromBundleTag fetches the bundle tag (sha256-xxx, no .sig suffix) and verifies via
// new-style sigstore bundle layers.
//
// Returns (true, nil) if a bundle was successfully verified, (true, err) if bundle layers were
// found but verification failed, and (false, nil) if the tag was not found or had no bundle layers.
//
//nolint:gocyclo
func verifyFromBundleTag(
	ctx context.Context, logger *zap.Logger, resolver remotes.Resolver, imageRef name.Digest, artifactPolicyOption verify.ArtifactPolicyOption, co cosign.CheckOpts,
) (bool, *VerifyResult, error) {
	bundleTag := strings.ReplaceAll(imageRef.DigestStr(), ":", "-")

	logger.Debug("resolving bundle tag", zap.String("bundleTag", bundleTag))

	resolvedName, desc, err := resolver.Resolve(ctx, imageRef.Repository.Name()+":"+bundleTag)
	if err != nil {
		logger.Debug("bundle tag not found", zap.String("bundleTag", bundleTag), zap.Error(err))

		return false, nil, nil
	}

	logger.Debug("resolved bundle tag", zap.String("name", resolvedName), zap.String("media_type", desc.MediaType))

	fetcher, err := resolver.Fetcher(ctx, resolvedName)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get fetcher for bundle tag: %w", err)
	}

	var bundleLayers []ocispec.Descriptor

	switch desc.MediaType {
	case ocispec.MediaTypeImageManifest:
		manifest, err := fetchManifest(ctx, fetcher, desc)
		if err != nil {
			return false, nil, fmt.Errorf("failed to fetch bundle manifest: %w", err)
		}

		for _, layer := range manifest.Layers[:min(maxLayers, len(manifest.Layers))] {
			if strings.HasPrefix(layer.MediaType, sigstoreBundleMediaTypePrefix) {
				bundleLayers = append(bundleLayers, layer)
			}
		}
	case ocispec.MediaTypeImageIndex:
		// The bundle tag may be an OCI image index wrapping individual bundle manifests.
		// Walk each manifest entry and collect bundle layers from all of them.
		index, err := fetchIndex(ctx, fetcher, desc)
		if err != nil {
			return false, nil, fmt.Errorf("failed to fetch bundle index: %w", err)
		}

		for _, manifestDesc := range index.Manifests[:min(maxLayers, len(index.Manifests))] {
			if manifestDesc.MediaType != ocispec.MediaTypeImageManifest {
				continue
			}

			manifest, err := fetchManifest(ctx, fetcher, manifestDesc)
			if err != nil {
				logger.Debug("failed to fetch manifest from bundle index", zap.String("digest", manifestDesc.Digest.String()), zap.Error(err))

				continue
			}

			for _, layer := range manifest.Layers[:min(maxLayers, len(manifest.Layers))] {
				if strings.HasPrefix(layer.MediaType, sigstoreBundleMediaTypePrefix) {
					bundleLayers = append(bundleLayers, layer)
				}
			}
		}
	default:
		logger.Debug("unexpected media type for bundle tag, skipping", zap.String("media_type", desc.MediaType))

		return false, nil, nil
	}

	if len(bundleLayers) == 0 {
		logger.Debug("no bundle layers found in bundle tag")

		return false, nil, nil
	}

	result, err := verifyBundleLayers(ctx, logger, fetcher, bundleLayers, artifactPolicyOption, co)

	return true, result, err
}

// verifyFromLegacySigTag fetches the legacy .sig tag (sha256-xxx.sig) and verifies via legacy
// cosign signature layers.
func verifyFromLegacySigTag(ctx context.Context, logger *zap.Logger, resolver remotes.Resolver, imageRef name.Digest, imageDigest v1.Hash, co cosign.CheckOpts) (*VerifyResult, error) {
	signatureTag := strings.ReplaceAll(imageRef.DigestStr(), ":", "-") + ".sig"

	logger.Debug("resolving .sig tag", zap.String("signatureTag", signatureTag))

	resolvedName, desc, err := resolver.Resolve(ctx, imageRef.Repository.Name()+":"+signatureTag)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve .sig tag %s: %w", signatureTag, err)
	}

	logger.Debug("resolved .sig manifest", zap.String("name", resolvedName), zap.String("media_type", desc.MediaType))

	if desc.MediaType != ocispec.MediaTypeImageManifest {
		return nil, fmt.Errorf("unexpected media type for .sig manifest: %s", desc.MediaType)
	}

	fetcher, err := resolver.Fetcher(ctx, resolvedName)
	if err != nil {
		return nil, fmt.Errorf("failed to get fetcher for .sig manifest: %w", err)
	}

	manifest, err := fetchManifest(ctx, fetcher, desc)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch .sig manifest: %w", err)
	}

	legacyLayers := manifest.Layers[:min(maxLayers, len(manifest.Layers))]

	return verifyLegacyLayers(ctx, logger, fetcher, legacyLayers, imageDigest, co)
}

// verifyBundleLayers verifies layers from a .sig manifest that carry sigstore bundle JSON directly.
func verifyBundleLayers(
	ctx context.Context, logger *zap.Logger, fetcher remotes.Fetcher, layers []ocispec.Descriptor, artifactPolicyOption verify.ArtifactPolicyOption, co cosign.CheckOpts,
) (*VerifyResult, error) {
	var lastErr error

	for i, layer := range layers {
		b, err := fetchBundleFromLayer(ctx, fetcher, layer)
		if err != nil {
			logger.Debug("failed to fetch bundle layer", zap.Int("layer", i), zap.Error(err))
			lastErr = err

			continue
		}

		co.NewBundleFormat = true

		if _, err := cosign.VerifyNewBundle(ctx, &co, artifactPolicyOption, b); err != nil {
			logger.Debug("bundle layer verification failed", zap.Int("layer", i), zap.Error(err))
			lastErr = err

			continue
		}

		logger.Debug("bundle layer verified", zap.Int("layer", i))

		return &VerifyResult{
			Message: fmt.Sprintf("verified via bundle layer with digest %s", layer.Digest.String()),
		}, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("no valid bundle layer: %w", lastErr)
	}

	return nil, errors.New("no valid bundle layers")
}

// verifyLegacyLayers verifies layers from a .sig manifest as legacy cosign signatures.
func verifyLegacyLayers(ctx context.Context, logger *zap.Logger, fetcher remotes.Fetcher, layers []ocispec.Descriptor, imageDigest v1.Hash, co cosign.CheckOpts) (*VerifyResult, error) {
	var lastErr error

	for i, layer := range layers {
		sig, err := buildLegacySignature(ctx, fetcher, layer)
		if err != nil {
			logger.Debug("failed to build legacy signature from layer", zap.Int("layer", i), zap.Error(err))
			lastErr = err

			continue
		}

		co.NewBundleFormat = false

		bundleVerified, err := cosign.VerifyImageSignature(ctx, sig, imageDigest, &co)
		if err != nil {
			logger.Debug("legacy signature verification failed", zap.Int("layer", i), zap.Error(err))
			lastErr = err

			continue
		}

		logger.Debug("legacy signature verified", zap.Int("layer", i), zap.Bool("bundle_verified", bundleVerified))

		return &VerifyResult{
			Message: fmt.Sprintf("verified via legacy signature (bundle verified %v) layer with digest %s", bundleVerified, layer.Digest.String()),
		}, nil
	}

	if lastErr != nil {
		return nil, fmt.Errorf("no valid legacy signature: %w", lastErr)
	}

	return nil, errors.New("no legacy signatures found")
}

// fetchManifest fetches and decodes an OCI image manifest descriptor.
//
//nolint:dupl // not a duplicate!
func fetchManifest(ctx context.Context, fetcher remotes.Fetcher, desc ocispec.Descriptor) (ocispec.Manifest, error) {
	rc, err := fetcher.Fetch(ctx, desc)
	if err != nil {
		return ocispec.Manifest{}, fmt.Errorf("failed to fetch: %w", err)
	}

	defer rc.Close() //nolint:errcheck

	var manifest ocispec.Manifest
	if err := json.NewDecoder(io.LimitReader(rc, maxManifestSize)).Decode(&manifest); err != nil {
		return ocispec.Manifest{}, fmt.Errorf("failed to decode manifest: %w", err)
	}

	return manifest, nil
}

// fetchIndex fetches and decodes an OCI image index descriptor.
//
//nolint:dupl // not a duplicate!
func fetchIndex(ctx context.Context, fetcher remotes.Fetcher, desc ocispec.Descriptor) (ocispec.Index, error) {
	rc, err := fetcher.Fetch(ctx, desc)
	if err != nil {
		return ocispec.Index{}, fmt.Errorf("failed to fetch: %w", err)
	}

	defer rc.Close() //nolint:errcheck

	var index ocispec.Index
	if err := json.NewDecoder(io.LimitReader(rc, maxManifestSize)).Decode(&index); err != nil {
		return ocispec.Index{}, fmt.Errorf("failed to decode index: %w", err)
	}

	return index, nil
}

// fetchBundleFromReferrer fetches a sigstore bundle stored as an OCI referrer.
//
// The referrer descriptor points to an OCI image manifest whose single layer contains the
// sigstore bundle JSON (media type application/vnd.dev.sigstore.bundle.v0.3+json).
func fetchBundleFromReferrer(ctx context.Context, fetcher remotes.Fetcher, referrer ocispec.Descriptor) (*sgbundle.Bundle, error) {
	manifest, err := fetchManifest(ctx, fetcher, referrer)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch bundle manifest: %w", err)
	}

	if len(manifest.Layers) != 1 {
		return nil, fmt.Errorf("expected exactly one layer in bundle manifest, got %d", len(manifest.Layers))
	}

	return fetchBundleFromLayer(ctx, fetcher, manifest.Layers[0])
}

// fetchBundleFromLayer fetches a sigstore bundle stored directly as a .sig manifest layer.
//
// The layer content is the sigstore bundle JSON.
func fetchBundleFromLayer(ctx context.Context, fetcher remotes.Fetcher, layer ocispec.Descriptor) (*sgbundle.Bundle, error) {
	rc, err := fetcher.Fetch(ctx, layer)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch bundle layer: %w", err)
	}

	defer rc.Close() //nolint:errcheck

	bundleBytes, err := io.ReadAll(io.LimitReader(rc, maxBundleSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read bundle layer: %w", err)
	}

	b := &sgbundle.Bundle{}
	if err := b.UnmarshalJSON(bundleBytes); err != nil {
		return nil, fmt.Errorf("failed to parse sigstore bundle: %w", err)
	}

	if !b.MinVersion("v0.3") {
		return nil, errors.New("bundle version too old (requires v0.3+)")
	}

	return b, nil
}

// buildLegacySignature constructs an oci.Signature from a cosign legacy signature layer.
//
// The layer content is the simple signing payload (the data that was signed). The cryptographic
// signature and optional certificate/chain/Rekor bundle are read from the layer annotations.
func buildLegacySignature(ctx context.Context, fetcher remotes.Fetcher, layer ocispec.Descriptor) (oci.Signature, error) {
	// Fetch the layer content: the simple signing payload (what was signed).
	rc, err := fetcher.Fetch(ctx, layer)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch signature layer: %w", err)
	}

	defer rc.Close() //nolint:errcheck

	payload, err := io.ReadAll(io.LimitReader(rc, maxPayloadSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read signature layer: %w", err)
	}

	// Base64-encoded cryptographic signature from layer annotation.
	b64sig, ok := layer.Annotations[static.SignatureAnnotationKey]
	if !ok {
		return nil, errors.New("missing signature annotation")
	}

	var staticOpts []static.Option

	// Certificate and intermediate chain for keyless (Fulcio-issued) signing.
	if certPEM, ok := layer.Annotations[static.CertificateAnnotationKey]; ok {
		chainPEM := layer.Annotations[static.ChainAnnotationKey]

		staticOpts = append(staticOpts, static.WithCertChain([]byte(certPEM), []byte(chainPEM)))
	}

	// Rekor transparency log bundle, if present.
	if bundleJSON, ok := layer.Annotations[static.BundleAnnotationKey]; ok {
		var rb bundle.RekorBundle
		if err := json.Unmarshal([]byte(bundleJSON), &rb); err != nil {
			return nil, fmt.Errorf("failed to parse Rekor bundle annotation: %w", err)
		}

		staticOpts = append(staticOpts, static.WithBundle(&rb))
	}

	return static.NewSignature(payload, b64sig, staticOpts...)
}
