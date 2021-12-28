// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package digitalocean

import (
	"context"
	stderrors "errors"
	"log"

	"github.com/talos-systems/go-procfs/procfs"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

const (
	// DigitalOceanExternalIPEndpoint displays all external addresses associated with the instance.
	DigitalOceanExternalIPEndpoint = "http://169.254.169.254/metadata/v1/interfaces/public/0/ipv4/address"
	// DigitalOceanHostnameEndpoint is the local endpoint for the hostname.
	DigitalOceanHostnameEndpoint = "http://169.254.169.254/metadata/v1/hostname"
	// DigitalOceanUserDataEndpoint is the local endpoint for the config.
	DigitalOceanUserDataEndpoint = "http://169.254.169.254/metadata/v1/user-data"
)

// DigitalOcean is the concrete type that implements the platform.Platform interface.
type DigitalOcean struct{}

// Name implements the platform.Platform interface.
func (d *DigitalOcean) Name() string {
	return "digital-ocean"
}

// Configuration implements the platform.Platform interface.
func (d *DigitalOcean) Configuration(ctx context.Context) ([]byte, error) {
	log.Printf("fetching machine config from: %q", DigitalOceanUserDataEndpoint)

	return download.Download(ctx, DigitalOceanUserDataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
}

// Mode implements the platform.Platform interface.
func (d *DigitalOcean) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (d *DigitalOcean) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("ttyS0").Append("tty0").Append("tty1"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
//
//nolint:gocyclo
func (d *DigitalOcean) NetworkConfiguration(ctx context.Context, ch chan<- *runtime.PlatformNetworkConfig) error {
	host, err := download.Download(ctx, DigitalOceanHostnameEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoHostname),
		download.WithErrorOnEmptyResponse(errors.ErrNoHostname))
	if err != nil && !stderrors.Is(err, errors.ErrNoHostname) {
		return err
	}

	extIP, err := download.Download(ctx, DigitalOceanExternalIPEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoExternalIPs),
		download.WithErrorOnEmptyResponse(errors.ErrNoExternalIPs))
	if err != nil && !stderrors.Is(err, errors.ErrNoExternalIPs) {
		return err
	}

	networkConfig := &runtime.PlatformNetworkConfig{}

	if len(host) > 0 {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(string(host)); err != nil {
			return err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	if len(extIP) > 0 {
		if ip, err := netaddr.ParseIP(string(extIP)); err == nil {
			networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
		}
	}

	select {
	case ch <- networkConfig:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}
