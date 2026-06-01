// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package security

import (
	"errors"
	"fmt"
	"strings"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// ImageVerificationConfigKind defines the ImageVerificationConfig configuration kind.
const ImageVerificationConfigKind = "ImageVerificationConfig"

func init() {
	registry.Register(ImageVerificationConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1":
			return &ImageVerificationConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.ImageVerificationConfig = &ImageVerificationConfigV1Alpha1{}
	_ config.Validator               = &ImageVerificationConfigV1Alpha1{}
)

// ImageVerificationConfigV1Alpha1 configures image signature verification policy.
//
//	examples:
//	  - value: exampleImageVerificationConfigV1Alpha1()
//	alias: ImageVerificationConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/ImageVerificationConfig
type ImageVerificationConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     List of verification rules.
	//     Rules are evaluated in order; first matching rule applies.
	ConfigRules ImageVerificationRules `yaml:"rules,omitempty"`
}

// ImageVerificationRules is a list of ImageVerificationRuleV1Alpha1.
//
//docgen:alias
type ImageVerificationRules []ImageVerificationRuleV1Alpha1

//docgen:nodoc
type indexedRule struct {
	i    int
	rule ImageVerificationRuleV1Alpha1
}

// Merge the network interface configuration intelligently.
func (r *ImageVerificationRules) Merge(other any) error {
	otherRules, ok := other.(ImageVerificationRules)
	if !ok {
		return fmt.Errorf("unexpected type for merge %T", other)
	}

	rulesByPattern := make(map[string]indexedRule)
	for i, rule := range *r {
		rulesByPattern[rule.RuleImagePattern] = indexedRule{i: i, rule: rule}
	}

	for _, otherRule := range otherRules {
		iRule, exists := rulesByPattern[otherRule.RuleImagePattern]
		if exists {
			// replace
			(*r)[iRule.i] = otherRule
		} else {
			// append
			*r = append(*r, otherRule)
		}
	}

	return nil
}

// ImageVerificationRuleV1Alpha1 defines a verification rule.
type ImageVerificationRuleV1Alpha1 struct {
	//   description: |
	//     Image reference pattern to match for this rule.
	//     Supports glob patterns, matches only on the image registry and repository, not on the tag or digest.
	//   examples:
	//     - value: >
	//         "docker.io/library/nginx"
	//     - value: >
	//         "registry.k8s.io/*"
	RuleImagePattern string `yaml:"image,omitempty"`

	//   description: |
	//     Skip verification for this image pattern (default: false).
	RuleSkip *bool `yaml:"skip,omitempty"`

	//   description: |
	//     Deny pulling images matching the pattern (default: false).
	RuleDeny *bool `yaml:"deny,omitempty"`

	//   description: |
	//     Keyless verifier configuration to use for this rule.
	RuleKeylessVerifier *ImageKeylessVerifierV1Alpha1 `yaml:"keyless,omitempty"`

	//   description: |
	//     Public key verifier configuration to use for this rule.
	RulePublicKeyVerifier *ImagePublicKeyVerifierV1Alpha1 `yaml:"publicKey,omitempty"`
}

// ImageKeylessVerifierV1Alpha1 configures a signature verification provider using Cosign keyless verification.
type ImageKeylessVerifierV1Alpha1 struct {
	//   description: |
	//     OIDC issuer URL for keyless verification.
	//   examples:
	//      - value: >
	//         "https://accounts.google.com"
	//      - value: >
	//         "https://token.actions.githubusercontent.com"
	KeylessIssuer string `yaml:"issuer,omitempty"`

	//   description: |
	//     Expected subject for keyless verification.
	//
	//     This is the identity (email, URI) that signed the image.
	KeylessSubject string `yaml:"subject,omitempty"`

	//   description: |
	//     Regex pattern for subject matching.
	//
	//     Use this instead of subject for flexible matching.
	//   examples:
	//       - value: >
	//           ".*@example\\.com"
	KeylessSubjectRegex string `yaml:"subjectRegex,omitempty"`
}

// ImagePublicKeyVerifierV1Alpha1 configures a signature verification provider using a static public key.
type ImagePublicKeyVerifierV1Alpha1 struct {
	//   description: |
	//     A public certificate in PEM format accepted for image signature verification.
	ConfigCertificate string `yaml:"certificate,omitempty"`
}

// NewImageVerificationConfigV1Alpha1 creates a new ImageVerificationConfig config document.
func NewImageVerificationConfigV1Alpha1() *ImageVerificationConfigV1Alpha1 {
	return &ImageVerificationConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       ImageVerificationConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleImageVerificationConfigV1Alpha1() *ImageVerificationConfigV1Alpha1 {
	cfg := NewImageVerificationConfigV1Alpha1()
	cfg.ConfigRules = []ImageVerificationRuleV1Alpha1{
		{
			RuleImagePattern: "registry.k8s.io/*",
			RuleKeylessVerifier: &ImageKeylessVerifierV1Alpha1{
				KeylessIssuer:  "https://accounts.google.com",
				KeylessSubject: "krel-trust@k8s-releng-prod.iam.gserviceaccount.com",
			},
		},
		{
			RuleImagePattern: "my-registry/*",
			RulePublicKeyVerifier: &ImagePublicKeyVerifierV1Alpha1{
				ConfigCertificate: `-----BEGIN CERTIFICATE-----
MII--Sample Value--
-----END CERTIFICATE-----`,
			},
		},
		{
			RuleImagePattern: "locahost:3000/*",
			RuleDeny:         new(true),
		},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *ImageVerificationConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo,cyclop
func (s *ImageVerificationConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	for i, rule := range s.ConfigRules {
		if rule.RuleImagePattern == "" {
			errs = errors.Join(errs, fmt.Errorf("rule %d: imagePattern must be specified", i))
		}

		if strings.ContainsRune(rule.RuleImagePattern, '@') {
			warnings = append(warnings, fmt.Sprintf("rule %d: imagePattern contains '@' but matching only applies to the image registry and repository, not the tag or digest", i))
		}

		// the ':' might be part of the registry (e.g. localhost:5000) so we cannot simply disallow it, but we can warn about it as it's a common mistake to include the tag in the pattern
		// warn if the `:` is after the first `/` (if any), which means it's likely part of the repository name or tag, not the registry
		if strings.ContainsRune(rule.RuleImagePattern, ':') {
			_, withoutRegistry, ok := strings.Cut(rule.RuleImagePattern, "/")
			if ok && strings.ContainsRune(withoutRegistry, ':') {
				warnings = append(warnings, fmt.Sprintf("rule %d: imagePattern contains ':' but matching only applies to the image registry and repository, not the tag or digest", i))
			}
		}

		if !strings.ContainsRune(rule.RuleImagePattern, '/') && rule.RuleImagePattern != "*" && rule.RuleImagePattern != "" {
			warnings = append(warnings, fmt.Sprintf("rule %d: imagePattern does not contain a '/', image references like 'nginx' are matched as 'docker.io/nginx' (normalized)", i))
		}

		skip := pointer.SafeDeref(rule.RuleSkip)
		deny := pointer.SafeDeref(rule.RuleDeny)
		hasRules := rule.RuleKeylessVerifier != nil || rule.RulePublicKeyVerifier != nil

		if skip && deny {
			errs = errors.Join(errs, fmt.Errorf("rule %d: skip and deny cannot both be true", i))
		}

		if (skip || deny) && hasRules {
			errs = errors.Join(errs, fmt.Errorf("rule %d: verifiers cannot be configured if skip or deny is true", i))
		}

		if !skip && !deny && !hasRules {
			errs = errors.Join(errs, fmt.Errorf("rule %d: at least one verifier must be configured", i))
		}

		if rule.RuleKeylessVerifier != nil && rule.RulePublicKeyVerifier != nil {
			errs = errors.Join(errs, fmt.Errorf("rule %d: only one of keyless or publicKey verifier can be configured", i))
		}

		if rule.RuleKeylessVerifier != nil {
			ruleVerifier := rule.RuleKeylessVerifier
			if ruleVerifier.KeylessIssuer == "" {
				errs = errors.Join(errs, fmt.Errorf("rule %d: verifier OIDC issuer must be specified", i))
			}

			if ruleVerifier.KeylessSubject == "" && ruleVerifier.KeylessSubjectRegex == "" {
				errs = errors.Join(errs,
					fmt.Errorf("rule %d: verifier subject or subjectRegex must be specified", i))
			}
		}

		if rule.RulePublicKeyVerifier != nil {
			ruleVerifier := rule.RulePublicKeyVerifier
			if ruleVerifier.ConfigCertificate == "" {
				errs = errors.Join(errs, fmt.Errorf("rule %d: verifier certificates must be specified", i))
			}
		}
	}

	return warnings, errs
}

// Rules implements config.ImageVerificationConfig interface.
func (s *ImageVerificationConfigV1Alpha1) Rules() []config.ImageVerificationRule {
	return xslices.Map(s.ConfigRules, func(r ImageVerificationRuleV1Alpha1) config.ImageVerificationRule {
		return &r
	})
}

// ImagePattern implements config.ImageVerificationRule interface.
func (r *ImageVerificationRuleV1Alpha1) ImagePattern() string {
	return r.RuleImagePattern
}

// Skip implements config.ImageVerificationRule interface.
func (r *ImageVerificationRuleV1Alpha1) Skip() bool {
	return pointer.SafeDeref(r.RuleSkip)
}

// Deny implements config.ImageVerificationRule interface.
func (r *ImageVerificationRuleV1Alpha1) Deny() bool {
	return pointer.SafeDeref(r.RuleDeny)
}

// VerifierKeyless implements config.ImageVerificationRule interface.
func (r *ImageVerificationRuleV1Alpha1) VerifierKeyless() config.ImageKeylessVerifier {
	if r.RuleKeylessVerifier == nil {
		return nil
	}

	return r.RuleKeylessVerifier
}

// VerifierPublicKey implements config.ImageVerificationRule interface.
func (r *ImageVerificationRuleV1Alpha1) VerifierPublicKey() config.ImagePublicKeyVerifier {
	if r.RulePublicKeyVerifier == nil {
		return nil
	}

	return r.RulePublicKeyVerifier
}

// Issuer implements config.ImageVerifierKeyless interface.
func (k *ImageKeylessVerifierV1Alpha1) Issuer() string {
	return k.KeylessIssuer
}

// Subject implements config.ImageVerifierKeyless interface.
func (k *ImageKeylessVerifierV1Alpha1) Subject() string {
	return k.KeylessSubject
}

// SubjectRegex implements config.ImageVerifierKeyless interface.
func (k *ImageKeylessVerifierV1Alpha1) SubjectRegex() string {
	return k.KeylessSubjectRegex
}

// Certificate implements config.ImagePublicKeyVerifier interface.
func (p *ImagePublicKeyVerifierV1Alpha1) Certificate() string {
	return p.ConfigCertificate
}
