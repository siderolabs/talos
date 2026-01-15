// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:revive
package security

import (
	"iter"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
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
	// Action is the action for matching images.
	Verify bool `yaml:"verify" protobuf:"3"`
	// KeylessVerifier is the keyless verifier configuration to use.
	KeylessVerifier *ImageKeylessVerifierSpec `yaml:"keylessVerifier,omitempty" protobuf:"4"`
	// PublicKeyVerifier is the public key verifier configuration to use.
	PublicKeyVerifier *ImagePublicKeyVerifierSpec `yaml:"publicKeyVerifier,omitempty" protobuf:"5"`
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
	// RekorURL is the Rekor transparency log URL.
	RekorURL string `yaml:"rekorURL" protobuf:"4"`
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
				Name:     "Enabled",
				JSONPath: "{.enabled}",
			},
		},
	}
}

// MatchImageVerificationRule matches the given image reference against the provided rules and returns the first matching rule.
func MatchImageVerificationRule(ref string, rules iter.Seq[*ImageVerificationRule]) *ImageVerificationRule {
	for rule := range rules {
		if rule.TypedSpec().ImagePattern != "" && glob.Glob(rule.TypedSpec().ImagePattern, ref) {
			return rule
		}
	}

	return nil
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic(ImageVerificationRuleType, &ImageVerificationRule{})
	if err != nil {
		panic(err)
	}
}
