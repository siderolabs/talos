// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package providers

import (
	"context"
	"fmt"

	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/docker"
	"github.com/siderolabs/talos/pkg/provision/providers/remote"
)

const (
	// QemuProviderName is the name of the qemu provider.
	QemuProviderName = "qemu"
	// DockerProviderName is the name of the docker provider.
	DockerProviderName = "docker"
	// RemoteProviderName is the name of the remote (gRPC) provider.
	RemoteProviderName = remote.ProviderName
)

// FactoryOption configures Factory construction.
type FactoryOption func(*factoryOptions)

type factoryOptions struct {
	remoteEndpoint string
}

// WithRemoteEndpoint sets the gRPC endpoint for the remote provisioner.
func WithRemoteEndpoint(endpoint string) FactoryOption {
	return func(o *factoryOptions) {
		o.remoteEndpoint = endpoint
	}
}

// Factory instantiates provision provider by name.
func Factory(ctx context.Context, name string, opts ...FactoryOption) (provision.Provisioner, error) {
	if err := IsValidProvider(name); err != nil {
		return nil, err
	}

	options := factoryOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	switch name {
	case DockerProviderName:
		return docker.NewProvisioner(ctx)
	case QemuProviderName:
		return newQemu(ctx)
	case RemoteProviderName:
		if options.remoteEndpoint == "" {
			return nil, fmt.Errorf("%q provisioner requires WithRemoteEndpoint option", RemoteProviderName)
		}

		return remote.NewProvisioner(ctx, options.remoteEndpoint)
	}

	panic("unknown valid provisioner")
}

// IsValidProvider returns an error if the passed provider doesn't exist.
func IsValidProvider(name string) error {
	switch name {
	case QemuProviderName, DockerProviderName, RemoteProviderName:
		return nil
	}

	return fmt.Errorf("unsupported provisioner %q", name)
}
