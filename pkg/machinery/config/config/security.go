// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

// TrustedRootsConfig defines the interface to access trusted roots configuration.
type TrustedRootsConfig interface {
	ExtraTrustedRootCertificates() []string
}

// WrapTrustedRootsConfig wraps a list of TrustedRootsConfig into a single TrustedRootsConfig aggregating the results.
func WrapTrustedRootsConfig(configs ...TrustedRootsConfig) TrustedRootsConfig {
	return trustedRootConfigWrapper(configs)
}

type trustedRootConfigWrapper []TrustedRootsConfig

func (w trustedRootConfigWrapper) ExtraTrustedRootCertificates() []string {
	return aggregateValues(w, func(c TrustedRootsConfig) []string {
		return c.ExtraTrustedRootCertificates()
	})
}

// ImageVerificationConfig specifies image signature verification policy.
type ImageVerificationConfig interface {
	// Rules returns the list of verification rules.
	Rules() []ImageVerificationRule
}

// ImageVerificationRule represents a rule for image verification.
type ImageVerificationRule interface {
	// ImagePattern returns the image name pattern.
	ImagePattern() string
	// Action returns the action for matching images.
	Verify() bool
	// VerifierKeyless returns the keyless verifier to use for this rule (optional).
	VerifierKeyless() ImageKeylessVerifier
	// VerifierPublicKey returns the public key verifier to use for this rule (optional).
	VerifierPublicKey() ImagePublicKeyVerifier
}

// ImageKeylessVerifier represents a signature verification provider with keyless verification.
type ImageKeylessVerifier interface {
	// Issuer returns the OIDC issuer URL.
	Issuer() string
	// Subject returns the expected subject (email, URI, etc).
	Subject() string
	// SubjectRegex returns the regex pattern for subject matching.
	SubjectRegex() string
	// RekorURL returns the Rekor transparency log URL.
	RekorURL() string
}

// ImagePublicKeyVerifier represents a signature verification provider with static public key.
type ImagePublicKeyVerifier interface {
	// Certificate returns a public certificate in PEM format accepted for image signature verification.
	Certificate() string
}
