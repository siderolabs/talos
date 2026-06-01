// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package containermode contains a utility function to detect if Talos is running in a container.
package containermode

import (
	"os"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// InContainer checks whether or not Talos is running in a container.
func InContainer() bool {
	if _, err := os.Stat(constants.ContainerMarkerFilePath); err == nil {
		return true
	}

	return false
}
