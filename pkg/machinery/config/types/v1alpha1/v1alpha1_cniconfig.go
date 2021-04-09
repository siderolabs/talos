// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

// Name implements the config.CNI interface.
func (c *CNIConfig) Name() string {
	return c.CNIName
}

// URLs implements the config.CNI interface.
func (c *CNIConfig) URLs() []string {
	return c.CNIUrls
}
