/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package container

import (
	"encoding/base64"
	"errors"
	"net"
	"os"

	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// Container is a platform for installing Talos via an Container image.
type Container struct{}

// Name implements the platform.Platform interface.
func (c *Container) Name() string {
	return "Container"
}

// Configuration implements the platform.Platform interface.
func (c *Container) Configuration() ([]byte, error) {
	s, ok := os.LookupEnv("USERDATA")
	if !ok {
		return nil, errors.New("missing USERDATA environment variable")
	}

	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}

	return decoded, nil
}

// Hostname implements the platform.Platform interface.
func (c *Container) Hostname() (hostname []byte, err error) {
	return nil, nil
}

// Mode implements the platform.Platform interface.
func (c *Container) Mode() runtime.Mode {
	return runtime.Container
}

// ExternalIPs provides any external addresses assigned to the instance
func (c *Container) ExternalIPs() (addrs []net.IP, err error) {
	return addrs, err
}
