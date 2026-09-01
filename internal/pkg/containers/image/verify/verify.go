// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package verify provides functionality to verify container images against configured verification policies.
package verify

import (
	"context"
	"crypto"
	"fmt"
	"time"

	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/errdefs"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/distribution/reference"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	sigsig "github.com/sigstore/sigstore/pkg/signature"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/pkg/containers/image/imageref"
	ourcosign "github.com/siderolabs/talos/internal/pkg/containers/image/verify/internal/cosign"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/security"
)

// TagFetcher is the fallback used when a resolver's digest-based manifest fetch
// returns NotFound; see [ourcosign.TagFetcher].
type TagFetcher = ourcosign.TagFetcher

// ImageSignature verifies image signature within Talos source code.
//
// The image reference is normalized with [imageref] before anything else: the verification rules
// are matched against the canonical reference, and the digested reference returned on success is
// in the same namespace, so that the image which was verified is the image which gets pulled.
//
// tagFetcher (optional) is invoked when the resolver's digest-based manifest
// fetch returns NotFound — see [TagFetcher].
//
//nolint:gocyclo
func ImageSignature(
	ctx context.Context, logger *zap.Logger, resources state.State, resolver remotes.Resolver, tagFetcher TagFetcher, imageRef string,
) (*machine.ImageServiceVerifyResponse, error) {
	logger = logger.With(zap.String("image_ref", imageRef))

	namedRef, err := imageref.Parse(imageRef)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "image reference is invalid: %s", err)
	}

	normalizedRef := namedRef.String()

	logger = logger.With(zap.String("normalized_image_ref", normalizedRef))

	ruleMatcher, err := NewRuleMatcher(ctx, resources)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create image verification rule matcher: %s", err)
	}

	logger.Debug("finding matching image verification rule for image reference")

	matchedRule, err := ruleMatcher(normalizedRef)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to match image verification rules: %s", err)
	}

	if matchedRule == nil {
		logger.Info("no matched image verification rule, allowing by default")

		return &machine.ImageServiceVerifyResponse{
			Verified: false,
			Message:  "no matched rule",
		}, nil
	}

	if matchedRule.TypedSpec().Deny {
		logger.Info("verification denied by matched rule", zap.String("rule_id", matchedRule.Metadata().ID()))

		return nil, status.Errorf(codes.PermissionDenied, "verification denied by matched rule (%s)", matchedRule.Metadata().ID())
	}

	if matchedRule.TypedSpec().Skip {
		logger.Info("verification skipped by matched rule", zap.String("rule_id", matchedRule.Metadata().ID()))

		return &machine.ImageServiceVerifyResponse{
			Verified: false,
			Message:  fmt.Sprintf("verification skipped by matched rule (%s)", matchedRule.Metadata().ID()),
		}, nil
	}

	// resolve the image reference to a digest reference if needed
	digestRef, ok := namedRef.(reference.Canonical)
	if !ok {
		_, desc, err := resolver.Resolve(ctx, normalizedRef)
		if err != nil {
			if errdefs.IsNotFound(err) {
				logger.Info("image reference not found during resolution", zap.Error(err))

				return nil, status.Errorf(codes.NotFound, "image reference not found during resolution: %s", err)
			}

			return nil, status.Errorf(codes.Internal, "failed to resolve image reference: %s", err)
		}

		digestRef, err = reference.WithDigest(reference.TrimNamed(namedRef), desc.Digest)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to construct digest reference: %s", err)
		}
	}

	// convert the rule to cosign check opts
	checkOpts, err := cosignCheckOptsFromRule(ctx, matchedRule.TypedSpec(), resources)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert verification rule to cosign check options: %s", err)
	}

	result, err := ourcosign.VerifyImage(ctx, logger, resolver, tagFetcher, digestRef, checkOpts)
	if err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "image verification failed: %s", err)
	}

	return &machine.ImageServiceVerifyResponse{
		Verified:         true,
		Message:          result.Message,
		DigestedImageRef: digestRef.String(),
	}, nil
}

// tufTimeout is the timeout for fetching TUF trusted root metadata during keyless verification.
const tufTimeout = 15 * time.Second

func getTrustedRoot(ctx context.Context, resources state.State) (root.TrustedMaterial, error) {
	ctx, cancel := context.WithTimeout(ctx, tufTimeout)
	defer cancel()

	r, err := resources.WatchFor(
		ctx, security.NewTUFTrustedRoot(security.TrustedRootID).Metadata(),
		state.WithEventTypes(state.Created, state.Updated),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to watch for TUF trusted root: %w", err)
	}

	tufData, ok := r.(*security.TUFTrustedRoot)
	if !ok {
		return nil, fmt.Errorf("unexpected resource type for TUF trusted root: %T", r)
	}

	return root.NewTrustedRootFromJSON([]byte(tufData.TypedSpec().JSONData))
}

func cosignCheckOptsFromRule(ctx context.Context, rule *security.ImageVerificationRuleSpec, resources state.State) (cosign.CheckOpts, error) {
	switch {
	case rule.KeylessVerifier != nil:
		trustedRoot, err := getTrustedRoot(ctx, resources)
		if err != nil {
			return cosign.CheckOpts{}, fmt.Errorf("failed to get trusted root: %w", err)
		}

		return cosign.CheckOpts{
			TrustedMaterial: trustedRoot,
			Identities: []cosign.Identity{
				{
					Subject:       rule.KeylessVerifier.Subject,
					SubjectRegExp: rule.KeylessVerifier.SubjectRegex,
					Issuer:        rule.KeylessVerifier.Issuer,
				},
			},
		}, nil
	case rule.PublicKeyVerifier != nil:
		pub, err := cryptoutils.UnmarshalPEMToPublicKey([]byte(rule.PublicKeyVerifier.Certificate))
		if err != nil {
			return cosign.CheckOpts{}, fmt.Errorf("failed to unmarshal public key: %w", err)
		}

		verifier, err := sigsig.LoadVerifier(pub, crypto.SHA256)
		if err != nil {
			return cosign.CheckOpts{}, fmt.Errorf("failed to load public key: %w", err)
		}

		return cosign.CheckOpts{
			Offline: true,
			// This is required on top of Offline to avoid tlog verification,
			// since we don't supply trusted roots.
			IgnoreTlog:  true,
			SigVerifier: verifier,
		}, nil
	default:
		return cosign.CheckOpts{}, fmt.Errorf("unsupported verifier type in rule")
	}
}
