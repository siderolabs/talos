// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import "net/netip"

// KubespanConfig defines the interface to access KubeSpan configuration.
type KubespanConfig interface {
	ExtraAnnouncedEndpoints() []netip.AddrPort
}

// WrapKubespanConfig wraps a list of KubespanConfig into a single KubespanConfig aggregating the results.
func WrapKubespanConfig(configs ...KubespanConfig) KubespanConfig {
	return kubespanConfigWrapper(configs)
}

type kubespanConfigWrapper []KubespanConfig

func (w kubespanConfigWrapper) ExtraAnnouncedEndpoints() []netip.AddrPort {
	return aggregateValues(w, func(c KubespanConfig) []netip.AddrPort {
		return c.ExtraAnnouncedEndpoints()
	})
}
