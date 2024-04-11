// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package files provides internal methods to container platform to read files.
package files

import (
	"bytes"
	"os"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// ReadHostname reads and parses /etc/hostname file.
func ReadHostname(path string) (network.HostnameSpecSpec, error) {
	hostname, err := os.ReadFile(path)
	if err != nil {
		return network.HostnameSpecSpec{}, err
	}

	hostname = bytes.TrimSpace(hostname)

	hostnameSpec := network.HostnameSpecSpec{
		ConfigLayer: network.ConfigPlatform,
	}

	if err = hostnameSpec.ParseFQDN(string(hostname)); err != nil {
		return network.HostnameSpecSpec{}, err
	}

	return hostnameSpec, nil
}
