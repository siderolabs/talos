// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

// Name implements the config.AdmissionPlugin interface.
func (a *AdmissionPluginConfig) Name() string {
	return a.PluginName
}

// Configuration implements the config.AdmissionPlugin interface.
func (a *AdmissionPluginConfig) Configuration() map[string]any {
	return a.PluginConfiguration.Object
}
