// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package gcp

import (
	"context"
	"strings"

	"cloud.google.com/go/compute/metadata"
	"github.com/talos-systems/go-procfs/procfs"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
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

	if strings.TrimSpace(userdata) == "" {
		return nil, errors.ErrNoConfigSource
	}

	return []byte(userdata), nil
}

// Mode implements the platform.Platform interface.
func (g *GCP) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (g *GCP) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("ttyS0"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (g *GCP) NetworkConfiguration(ctx context.Context, ch chan<- *runtime.PlatformNetworkConfig) error {
	networkConfig := &runtime.PlatformNetworkConfig{}

	hostname, err := metadata.Hostname()
	if err != nil {
		return err
	}

	if hostname != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err = hostnameSpec.ParseFQDN(hostname); err != nil {
			return err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	externalIP, err := metadata.ExternalIP()
	if err != nil {
		if _, ok := err.(metadata.NotDefinedError); !ok {
			return err
		}
	}

	if externalIP != "" {
		ip, err := netaddr.ParseIP(externalIP)
		if err != nil {
			return err
		}

		networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
	}

	select {
	case ch <- networkConfig:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}
