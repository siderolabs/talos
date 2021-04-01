// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

// Enabled implements the config.ExternalCloudProvider interface.
func (ecp *ExternalCloudProviderConfig) Enabled() bool {
	return ecp.ExternalEnabled
}

// ManifestURLs implements the config.ExternalCloudProvider interface.
func (ecp *ExternalCloudProviderConfig) ManifestURLs() []string {
	return ecp.ExternalManifests
}
