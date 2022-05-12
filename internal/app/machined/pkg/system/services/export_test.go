// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import "github.com/containerd/containerd/oci"

// GetOCIOptions gets all OCI options from an Extension.
func (svc *Extension) GetOCIOptions() []oci.SpecOpts {
	return svc.getOCIOptions()
}
