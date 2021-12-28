// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nocloud

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"

	"github.com/talos-systems/go-procfs/procfs"
	yaml "gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// Nocloud is the concrete type that implements the runtime.Platform interface.
type Nocloud struct{}

// Name implements the runtime.Platform interface.
func (n *Nocloud) Name() string {
	return "nocloud"
}

// ParseMetadata converts nocloud metadata to platform network config.
func (n *Nocloud) ParseMetadata(unmarshalledNetworkConfig *NetworkConfig, hostname string) (*runtime.PlatformNetworkConfig, error) {
	networkConfig := &runtime.PlatformNetworkConfig{}

	if hostname != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(hostname); err != nil {
			return nil, err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	switch unmarshalledNetworkConfig.Version {
	case 1:
		if err := n.applyNetworkConfigV1(unmarshalledNetworkConfig, networkConfig); err != nil {
			return nil, err
		}
	case 2:
		if err := n.applyNetworkConfigV2(unmarshalledNetworkConfig, networkConfig); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("network-config metadata version=%d is not supported", unmarshalledNetworkConfig.Version)
	}

	return networkConfig, nil
}

// Configuration implements the runtime.Platform interface.
func (n *Nocloud) Configuration(ctx context.Context) ([]byte, error) {
	_, _, machineConfigDl, _, err := n.acquireConfig(ctx) //nolint:dogsled
	if err != nil {
		return nil, err
	}

	if bytes.HasPrefix(machineConfigDl, []byte("#cloud-config")) {
		return nil, errors.ErrNoConfigSource
	}

	return machineConfigDl, nil
}

// Mode implements the runtime.Platform interface.
func (n *Nocloud) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (n *Nocloud) KernelArgs() procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
//
//nolint:gocyclo
func (n *Nocloud) NetworkConfiguration(ctx context.Context, ch chan<- *runtime.PlatformNetworkConfig) error {
	metadataConfigDl, metadataNetworkConfigDl, _, hostname, err := n.acquireConfig(ctx)
	if stderrors.Is(err, errors.ErrNoConfigSource) {
		err = nil
	}

	if err != nil {
		return err
	}

	if metadataConfigDl == nil && metadataNetworkConfigDl == nil && hostname == "" {
		// no data, use cached network configuration if available
		return nil
	}

	var (
		unmarshalledMetadataConfig MetadataConfig
		unmarshalledNetworkConfig  NetworkConfig
	)

	if metadataConfigDl != nil {
		_ = yaml.Unmarshal(metadataConfigDl, &unmarshalledMetadataConfig) //nolint:errcheck
	}

	if metadataNetworkConfigDl != nil {
		if err = yaml.Unmarshal(metadataNetworkConfigDl, &unmarshalledNetworkConfig); err != nil {
			return err
		}
	}

	if hostname == "" {
		hostname = unmarshalledMetadataConfig.Hostname
	}

	networkConfig, err := n.ParseMetadata(&unmarshalledNetworkConfig, hostname)
	if err != nil {
		return err
	}

	select {
	case ch <- networkConfig:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}
