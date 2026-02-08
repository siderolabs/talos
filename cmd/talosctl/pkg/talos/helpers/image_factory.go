// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"fmt"
	"strings"

	"github.com/siderolabs/talos/pkg/images"
)

// BuildImageFactoryURL builds an installer image reference of the form
// <factory>/<platform>-installer[-secureboot]/<schematic ID>:<version>.
//
// An empty schematic is substituted with the default (empty) schematic ID.
func BuildImageFactoryURL(factory, schematic, version, platform string, secureBoot bool) string {
	if schematic == "" {
		schematic = images.DefaultInstallerImageSchematic
	}

	installerType := platform + "-installer"
	if secureBoot {
		installerType += "-secureboot"
	}

	return fmt.Sprintf("%s/%s/%s:v%s", factory, installerType, schematic, strings.TrimPrefix(version, "v"))
}
