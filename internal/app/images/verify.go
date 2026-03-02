// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package images

import (
	"context"
	"crypto"
	"fmt"
	"sync"

	"github.com/containerd/errdefs"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	"github.com/sigstore/cosign/v3/pkg/signature"
	"github.com/sigstore/sigstore-go/pkg/root"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	ourcosign "github.com/siderolabs/talos/internal/app/images/internal/cosign"
	"github.com/siderolabs/talos/internal/pkg/containers/image"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
	"github.com/siderolabs/talos/pkg/machinery/resources/security"
)

// ImageVerify verifies an image against the configured verification policy.
//
// This endpoint is called by containerd before unpacking an image to ensure
// the image meets the verification requirements configured in the machine config.
//
// If no verification policy is configured, all images are allowed by default.
func (svc *Service) ImageVerify(ctx context.Context, req *machine.ImageServiceVerifyRequest) (*machine.ImageServiceVerifyResponse, error) {
	logger := svc.logger.With(zap.String("image_ref", req.GetImageRef()))

	inRef, err := name.ParseReference(req.GetImageRef())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "image reference is invalid: %s", err)
	}

	resources := svc.controller.Runtime().State().V1Alpha2().Resources()

	rules, err := safe.StateListAll[*security.ImageVerificationRule](ctx, resources)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list image verification rules: %s", err)
	}

	matchedRule := security.MatchImageVerificationRule(req.GetImageRef(), rules.All())
	if matchedRule == nil {
		logger.Info("no matched image verification rule, allowing by default")

		return &machine.ImageServiceVerifyResponse{
			Verified: false,
			Message:  "no matched rule",
		}, nil
	}

	if !matchedRule.TypedSpec().Verify {
		logger.Info("verification skipped by matched rule", zap.String("rule_id", matchedRule.Metadata().ID()))

		return &machine.ImageServiceVerifyResponse{
			Verified: false,
			Message:  fmt.Sprintf("verification skipped by matched rule (%s)", matchedRule.Metadata().ID()),
		}, nil
	}

	// build resolver
	registries, err := cri.RegistryBuilder(resources)(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to build registry configuration: %s", err)
	}

	resolver := image.NewResolver(registries)

	// resolve the image reference to a digest reference if needed
	var (
		digestRef name.Digest
		ok        bool
	)

	if digestRef, ok = inRef.(name.Digest); !ok {
		resolvedRef, desc, err := resolver.Resolve(ctx, inRef.String())
		if err != nil {
			if errdefs.IsNotFound(err) {
				logger.Info("image reference not found during resolution", zap.Error(err))

				return nil, status.Errorf(codes.NotFound, "image reference not found during resolution: %s", err)
			}

			return nil, status.Errorf(codes.Internal, "failed to resolve image reference: %s", err)
		}

		digestRef, err = name.NewDigest(resolvedRef + "@" + desc.Digest.String())
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to construct digest reference: %s", err)
		}
	}

	// convert the rule to cosign check opts
	checkOpts, err := cosignCheckOptsFromRule(matchedRule.TypedSpec())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert verification rule to cosign check options: %s", err)
	}

	result, err := ourcosign.VerifyImage(ctx, logger, resolver, digestRef, checkOpts)
	if err != nil {
		return nil, status.Errorf(codes.PermissionDenied, "image verification failed: %s", err)
	}

	return &machine.ImageServiceVerifyResponse{
		Verified: true,
		Message:  result.Message,
	}, nil
}

var (
	globalTrustedRoot root.TrustedMaterial
	globalMu          sync.Mutex
)

func getTrustedRoot() (root.TrustedMaterial, error) {
	globalMu.Lock()
	defer globalMu.Unlock()

	if globalTrustedRoot != nil {
		return globalTrustedRoot, nil
	}

	var err error

	globalTrustedRoot, err = cosign.TrustedRoot()
	if err != nil {
		globalTrustedRoot = nil
	}

	return globalTrustedRoot, err
}

func cosignCheckOptsFromRule(rule *security.ImageVerificationRuleSpec) (cosign.CheckOpts, error) {
	switch {
	case rule.KeylessVerifier != nil:
		trustedRoot, err := getTrustedRoot()
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
		verifier, err := signature.LoadPublicKeyRaw([]byte(rule.PublicKeyVerifier.Certificate), crypto.SHA256)
		if err != nil {
			return cosign.CheckOpts{}, fmt.Errorf("failed to load public key: %w", err)
		}

		return cosign.CheckOpts{
			Offline:     true,
			SigVerifier: verifier,
		}, nil
	default:
		return cosign.CheckOpts{}, fmt.Errorf("unsupported verifier type in rule")
	}
}
