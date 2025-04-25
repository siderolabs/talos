// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import "net/url"

// SideroLinkConfig defines the interface to access SideroLink configuration.
type SideroLinkConfig interface {
	APIUrl() *url.URL
	UniqueToken() string
}
