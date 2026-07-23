// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

// K8sInlineManifestConfigSignal implements the config.K8sInlineManifestConfig interface.
func (m ClusterInlineManifest) K8sInlineManifestConfigSignal() {}

// Name implements the config.K8sInlineManifestConfig interface.
func (m ClusterInlineManifest) Name() string {
	return m.InlineManifestName
}

// Contents implements the config.K8sInlineManifestConfig interface.
func (m ClusterInlineManifest) Contents() string {
	return m.InlineManifestContents
}
