// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gcp

import (
	"context"
	"net"
	"strings"

	"cloud.google.com/go/compute/metadata"
	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
)

// GCP is the concrete type that implements the platform.Platform interface.
type GCP struct{}

// Name implements the platform.Platform interface.
func (g *GCP) Name() string {
	return "gcp"
}

// Configuration implements the platform.Platform interface.
func (g *GCP) Configuration(ctx context.Context) ([]byte, error) {
	userdata, err := metadata.InstanceAttributeValue("user-data")
	if err != nil {
		if _, ok := err.(metadata.NotDefinedError); ok {
			return nil, errors.ErrNoConfigSource
		}

		return nil, err
	}

	userdata = strings.TrimSpace(userdata)

	if userdata == "" {
		return nil, errors.ErrNoConfigSource
	}

	return []byte(userdata), nil
}

// Hostname implements the platform.Platform interface.
func (g *GCP) Hostname(context.Context) (hostname []byte, err error) {
	host, err := metadata.Hostname()
	if err != nil {
		return nil, err
	}

	return []byte(host), nil
}

// Mode implements the platform.Platform interface.
func (g *GCP) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// ExternalIPs implements the runtime.Platform interface.
func (g *GCP) ExternalIPs(ctx context.Context) (addrs []net.IP, err error) {
	extIP, err := metadata.ExternalIP()
	if err != nil {
		if _, ok := err.(metadata.NotDefinedError); ok {
			return nil, nil
		}

		return nil, err
	}

	if addr := net.ParseIP(extIP); addr != nil {
		addrs = append(addrs, addr)
	}

	return addrs, nil
}

// KernelArgs implements the runtime.Platform interface.
func (g *GCP) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("ttyS0"),
	}
}
