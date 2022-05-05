// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import "github.com/siderolabs/go-pointer"

// Enabled implements config.KubernetesTalosAPIAccess.
func (c *KubernetesTalosAPIAccessConfig) Enabled() bool {
	if c == nil {
		return false
	}

	return pointer.SafeDeref(c.AccessEnabled)
}

// AllowedRoles implements config.KubernetesTalosAPIAccess.
func (c *KubernetesTalosAPIAccessConfig) AllowedRoles() []string {
	if c == nil {
		return nil
	}

	return c.AccessAllowedRoles
}

// AllowedKubernetesNamespaces implements config.KubernetesTalosAPIAccess.
func (c *KubernetesTalosAPIAccessConfig) AllowedKubernetesNamespaces() []string {
	if c == nil {
		return nil
	}

	return c.AccessAllowedKubernetesNamespaces
}
