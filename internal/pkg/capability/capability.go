// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package capability provides utility functions to work with capabilities.
package capability

import (
	"strings"

	"github.com/siderolabs/gen/slices"
	"kernel.org/pub/linux/libs/security/libcap/cap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// AllGrantableCapabilities returns list of capabilities that can be granted to the container based on
// process bounding capabilities.
func AllGrantableCapabilities() []string {
	capabilities := []string{}

	for v := cap.Value(0); v < cap.MaxBits(); v++ {
		if set, _ := cap.GetBound(v); set { //nolint:errcheck
			if slices.Contains(constants.DefaultDroppedCapabilities, func(capability string) bool {
				return capability == v.String()
			}) {
				continue
			}

			capabilities = append(capabilities, strings.ToUpper(v.String()))
		}
	}

	return capabilities
}
