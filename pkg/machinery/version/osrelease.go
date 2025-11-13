// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package version

//go:generate env CGO_ENABLED=0 go run ./gen.go

import (
	"fmt"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// OSRelease returns the contents of /etc/os-release.
func OSRelease() ([]byte, error) {
	var v string

	switch Tag {
	case "none":
		v = SHA
	default:
		v = Tag
	}

	return OSReleaseFor(Name, v)
}

// OSReleaseFor returns the contents of /etc/os-release for a given name and version.
func OSReleaseFor(name, version string) ([]byte, error) {
	return fmt.Appendf(nil, constants.OSReleaseTemplate, name, strings.ToLower(name), version), nil
}
