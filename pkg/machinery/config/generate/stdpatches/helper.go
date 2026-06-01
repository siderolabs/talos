// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package stdpatches

import "github.com/siderolabs/talos/pkg/machinery/config/configpatcher"

// PreparePatch is a helper function to prepare a patch for application to the machine configuration.
func PreparePatch(patch []byte, err error) (configpatcher.Patch, error) {
	if err != nil {
		return nil, err
	}

	return configpatcher.LoadPatch(patch)
}
