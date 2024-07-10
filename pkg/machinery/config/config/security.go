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
