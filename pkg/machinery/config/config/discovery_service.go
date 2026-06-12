// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import "net/url"

// DiscoveryServiceConfig defines the configuration for a discovery service.
type DiscoveryServiceConfig interface {
	// Name returns the name of the discovery service.
	Name() string

	// Endpoint returns the endpoint of the discovery service.
	Endpoint() *url.URL
}
