// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package capability provides utility functions to work with capabilities.
package capability

import (
	"strings"

	"github.com/siderolabs/gen/maps"
	"kernel.org/pub/linux/libs/security/libcap/cap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// AllCapabilitiesSet returns the set of all available capabilities.
//
// Returned capabilities are in UPPERCASE.
func AllCapabilitiesSet() map[string]struct{} {
	capabilities := make(map[string]struct{})

	for v := cap.Value(0); v < cap.MaxBits(); v++ {
		if set, _ := cap.GetBound(v); set { //nolint:errcheck
			capabilities[strings.ToUpper(v.String())] = struct{}{}
		}
	}

	return capabilities
}

// AllCapabilitiesSetLowercase returns the set of all available capabilities.
//
// Returned capabilities are in lowercase.
func AllCapabilitiesSetLowercase() map[string]struct{} {
	return maps.Map(AllCapabilitiesSet(),
		func(capability string, _ struct{}) (string, struct{}) {
			return strings.ToLower(capability), struct{}{}
		})
}

// AllGrantableCapabilities returns list of capabilities that can be granted to the container based on
// process bounding capabilities.
//
// Returned capabilities are in UPPERCASE.
func AllGrantableCapabilities() []string {
	allCapabilities := AllCapabilitiesSet()

	for dropped := range constants.DefaultDroppedCapabilities {
		delete(allCapabilities, strings.ToUpper(dropped))
	}

	return maps.Keys(allCapabilities)
}
