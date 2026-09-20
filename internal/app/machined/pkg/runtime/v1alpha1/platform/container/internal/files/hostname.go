// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package files provides internal methods to container platform to read files.
package files

import (
	"bytes"
	"errors"
	"os"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// ReadHostname reads and parses /etc/hostname file.
func ReadHostname(path string) (network.HostnameSpecSpec, error) {
	hostnameSpec := network.HostnameSpecSpec{
		ConfigLayer: network.ConfigPlatform,
	}

	hostname, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return hostnameSpec, nil
		}

		return network.HostnameSpecSpec{}, err
	}

	hostname = bytes.TrimSpace(hostname)

	if len(hostname) == 0 {
		return hostnameSpec, nil
	}

	if err = hostnameSpec.ParseFQDN(string(hostname)); err != nil {
		return network.HostnameSpecSpec{}, err
	}

	return hostnameSpec, nil
}
