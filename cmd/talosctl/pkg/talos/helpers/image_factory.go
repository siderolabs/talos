// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"fmt"
	"strings"
)

// BuildImageFactoryURL constructs an Image Factory URL from components.
func BuildImageFactoryURL(factory, schematic, version, platform string, secureBoot bool) string {
	// Clean version (remove 'v' prefix if present)
	version = strings.TrimPrefix(version, "v")

	// Determine installer type based on platform and secure boot
	installerType := platform + "-installer"
	if secureBoot {
		installerType += "-secureboot"
	}

	// Build URL based on whether we have a schematic
	var imageURL string
	if schematic == "" {
		imageURL = fmt.Sprintf("%s/%s:v%s", factory, installerType, version)
	} else {
		imageURL = fmt.Sprintf("%s/%s/%s:v%s", factory, installerType, schematic, version)
	}

	return imageURL
}

// Ternary is a helper for conditional strings.
func Ternary(condition bool, ifTrue, ifFalse string) string {
	if condition {
		return ifTrue
	}

	return ifFalse
}
