// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:revive
package security

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/ryanuber/go-glob"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// ImageVerificationRuleType is type of ImageVerificationRule resource.
const ImageVerificationRuleType = resource.Type("ImageVerificationRules.security.talos.dev")

// ImageVerificationRule represents ImageVerificationRule typed resource.
type ImageVerificationRule = typed.Resource[ImageVerificationRuleSpec, ImageVerificationRuleExtension]

// ImageVerificationRuleSpec represents a verification rule.
//
//gotagsrewrite:gen
type ImageVerificationRuleSpec struct {
	// ImagePattern is the image name pattern.
	ImagePattern string `yaml:"imagePattern,omitempty" protobuf:"2"`
	// Skip is the action for matching images.
	Skip bool `yaml:"skip" protobuf:"3"`
	// Deny is the action for matching images.
	Deny bool `yaml:"deny" protobuf:"4"`
	// KeylessVerifier is the keyless verifier configuration to use.
	KeylessVerifier *ImageKeylessVerifierSpec `yaml:"keylessVerifier,omitempty" protobuf:"5"`
	// PublicKeyVerifier is the public key verifier configuration to use.
	PublicKeyVerifier *ImagePublicKeyVerifierSpec `yaml:"publicKeyVerifier,omitempty" protobuf:"6"`
}

// ImageKeylessVerifierSpec represents a signature verification provider.
//
//gotagsrewrite:gen
type ImageKeylessVerifierSpec struct {
	// Issuer is the OIDC issuer URL.
	Issuer string `yaml:"issuer" protobuf:"1"`
	// Subject is the expected subject.
	Subject string `yaml:"subject,omitempty" protobuf:"2"`
	// SubjectRegex is a regex pattern for subject matching.
	SubjectRegex string `yaml:"subjectRegex,omitempty" protobuf:"3"`
}

// ImagePublicKeyVerifierSpec represents a signature verification provider with static public key.
//
//gotagsrewrite:gen
type ImagePublicKeyVerifierSpec struct {
	// Certificate is a public certificate in PEM format accepted for image signature verification.
	Certificate string `yaml:"certificate" protobuf:"1"`
}

// NewImageVerificationRule creates new ImageVerificationRule object.
func NewImageVerificationRule(id resource.ID) *ImageVerificationRule {
	return typed.NewResource[ImageVerificationRuleSpec, ImageVerificationRuleExtension](
		resource.NewMetadata(NamespaceName, ImageVerificationRuleType, id, resource.VersionUndefined),
		ImageVerificationRuleSpec{},
	)
}

// ImageVerificationRuleExtension is an auxiliary type for ImageVerificationRule resource.
type ImageVerificationRuleExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (ImageVerificationRuleExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ImageVerificationRuleType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Pattern",
				JSONPath: "{.imagePattern}",
			},
			{
				Name:     "Skip",
				JSONPath: "{.skip}",
			},
			{
				Name:     "Deny",
				JSONPath: "{.deny}",
			},
		},
	}
}

// ImageVerificationRuleMatchFunc is a function type for matching image references to verification rules.
type ImageVerificationRuleMatchFunc func(imageRef string) *ImageVerificationRule

// ImageVerificationRuleMatcher creates the matcher for the given image reference against the provided rules and returns the first matching rule.
func ImageVerificationRuleMatcher(ctx context.Context, st state.State) (ImageVerificationRuleMatchFunc, error) {
	rules, err := safe.StateListAll[*ImageVerificationRule](ctx, st)
	if err != nil {
		return nil, fmt.Errorf("failed to list image verification rules: %w", err)
	}

	return func(imageRef string) *ImageVerificationRule {
		for rule := range rules.All() {
			if rule.TypedSpec().ImagePattern != "" && glob.Glob(rule.TypedSpec().ImagePattern, imageRef) {
				return rule
			}
		}

		return nil
	}, nil
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(ImageVerificationRuleType, &ImageVerificationRule{})
	if err != nil {
		panic(err)
	}
}
