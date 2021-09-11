// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package openstack

import (
	"bytes"
	"context"
	"log"
	"net"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/download"
)

const (
	// OpenstackExternalIPEndpoint is the local Openstack endpoint for the external IP.
	OpenstackExternalIPEndpoint = "http://169.254.169.254/latest/meta-data/public-ipv4"

	// OpenstackHostnameEndpoint is the local Openstack endpoint for the hostname.
	OpenstackHostnameEndpoint = "http://169.254.169.254/latest/meta-data/hostname"

	// OpenstackUserDataEndpoint is the local Openstack endpoint for the config.
	OpenstackUserDataEndpoint = "http://169.254.169.254/latest/user-data"
)

// Openstack is the concrete type that implements the runtime.Platform interface.
type Openstack struct{}

// Name implements the runtime.Platform interface.
func (o *Openstack) Name() string {
	return "openstack"
}

// Configuration implements the runtime.Platform interface.
func (o *Openstack) Configuration(ctx context.Context) ([]byte, error) {
	log.Printf("fetching machine config from: %q", OpenstackUserDataEndpoint)

	machineConfigDl, err := download.Download(ctx, OpenstackUserDataEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoConfigSource),
		download.WithErrorOnEmptyResponse(errors.ErrNoConfigSource))
	if err != nil {
		return nil, err
	}

	// Some openstack setups does not allow you to change user-data,
	// so skip this case.
	if bytes.HasPrefix(machineConfigDl, []byte("#cloud-config")) {
		return nil, errors.ErrNoConfigSource
	}

	return machineConfigDl, nil
}

// Mode implements the runtime.Platform interface.
func (o *Openstack) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// Hostname implements the runtime.Platform interface.
func (o *Openstack) Hostname(ctx context.Context) (hostname []byte, err error) {
	log.Printf("fetching hostname from: %q", OpenstackHostnameEndpoint)

	hostname, err = download.Download(ctx, OpenstackHostnameEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoHostname),
		download.WithErrorOnEmptyResponse(errors.ErrNoHostname))
	if err != nil {
		// Platform cannot support this endpoint, or return timeout.
		// ApplyDynamicConfig can crash in this situation.
		log.Printf("failed to fetch hostname, ignored: %s", err)

		return nil, nil
	}

	return hostname, nil
}

// ExternalIPs implements the runtime.Platform interface.
func (o *Openstack) ExternalIPs(ctx context.Context) (addrs []net.IP, err error) {
	log.Printf("fetching externalIP from: %q", OpenstackExternalIPEndpoint)

	exIP, err := download.Download(ctx, OpenstackExternalIPEndpoint,
		download.WithErrorOnNotFound(errors.ErrNoExternalIPs),
		download.WithErrorOnEmptyResponse(errors.ErrNoExternalIPs))
	if err != nil {
		return nil, err
	}

	if addr := net.ParseIP(string(exIP)); addr != nil {
		addrs = append(addrs, addr)
	}

	return addrs, nil
}

// KernelArgs implements the runtime.Platform interface.
func (o *Openstack) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
	}
}
