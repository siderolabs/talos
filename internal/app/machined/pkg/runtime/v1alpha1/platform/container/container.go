// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container

import (
	"bytes"
	"context"
	"encoding/base64"
	"io/ioutil"
	"log"
	"net"
	"os"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
)

// Container is a platform for installing Talos via an Container image.
type Container struct{}

// Name implements the platform.Platform interface.
func (c *Container) Name() string {
	return "container"
}

// Configuration implements the platform.Platform interface.
func (c *Container) Configuration(context.Context) ([]byte, error) {
	log.Printf("fetching machine config from: USERDATA environment variable")

	s := os.Getenv("USERDATA")
	if s == "" {
		return nil, errors.ErrNoConfigSource
	}

	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}

	return decoded, nil
}

// Hostname implements the platform.Platform interface.
func (c *Container) Hostname(context.Context) (hostname []byte, err error) {
	hostname, err = ioutil.ReadFile("/etc/hostname")

	if err == nil {
		hostname = bytes.TrimSpace(hostname)
	}

	return
}

// Mode implements the platform.Platform interface.
func (c *Container) Mode() runtime.Mode {
	return runtime.ModeContainer
}

// ExternalIPs implements the runtime.Platform interface.
func (c *Container) ExternalIPs(context.Context) (addrs []net.IP, err error) {
	return addrs, err
}

// KernelArgs implements the runtime.Platform interface.
func (c *Container) KernelArgs() procfs.Parameters {
	return nil
}
