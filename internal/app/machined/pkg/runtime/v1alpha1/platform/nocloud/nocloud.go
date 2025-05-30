// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nocloud

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/go-procfs/procfs"
	yaml "gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/netutils"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// Nocloud is the concrete type that implements the runtime.Platform interface.
type Nocloud struct{}

// Name implements the runtime.Platform interface.
func (n *Nocloud) Name() string {
	return "nocloud"
}

// ParseMetadata converts nocloud metadata to platform network config.
func (n *Nocloud) ParseMetadata(ctx context.Context, unmarshalledNetworkConfig *NetworkConfig, st state.State, metadata *MetadataConfig) (*runtime.PlatformNetworkConfig, bool, error) {
	networkConfig := &runtime.PlatformNetworkConfig{}

	hostname := metadata.Hostname
	if hostname == "" {
		hostname = metadata.InternalDNS
	}

	if hostname != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(hostname); err != nil {
			return nil, false, err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	var (
		needsReconcile bool
		err            error
	)

	switch unmarshalledNetworkConfig.Version {
	case 1:
		if needsReconcile, err = n.applyNetworkConfigV1(ctx, unmarshalledNetworkConfig, st, networkConfig); err != nil {
			return nil, false, err
		}
	case 2:
		if needsReconcile, err = n.applyNetworkConfigV2(ctx, unmarshalledNetworkConfig, st, networkConfig); err != nil {
			return nil, false, err
		}
	default:
		return nil, false, fmt.Errorf("network-config metadata version=%d is not supported", unmarshalledNetworkConfig.Version)
	}

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:     n.Name(),
		Hostname:     hostname,
		InstanceID:   metadata.InstanceID,
		InstanceType: metadata.InstanceType,
		ProviderID:   metadata.ProviderID,
		Region:       metadata.Region,
		Zone:         metadata.Zone,
		InternalDNS:  metadata.InternalDNS,
		ExternalDNS:  metadata.ExternalDNS,
	}

	return networkConfig, needsReconcile, nil
}

// Configuration implements the runtime.Platform interface.
func (n *Nocloud) Configuration(ctx context.Context, r state.State) ([]byte, error) {
	_, machineConfigDl, _, err := n.acquireConfig(ctx, r)
	if err != nil {
		return nil, err
	}

	firstLine, rest, _ := bytes.Cut(machineConfigDl, []byte("\n"))
	firstLine = bytes.TrimSpace(firstLine)

	switch {
	case bytes.Equal(firstLine, []byte("#cloud-config")):
		// ignore cloud-config, Talos does not support it
		return nil, errors.ErrNoConfigSource
	case bytes.Equal(firstLine, []byte("#include")):
		return n.FetchInclude(ctx, rest, r)
	}

	return machineConfigDl, nil
}

// Mode implements the runtime.Platform interface.
func (n *Nocloud) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (n *Nocloud) KernelArgs(string, quirks.Quirks) procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("tty1").Append("ttyS0"),
		procfs.NewParameter(constants.KernelParamNetIfnames).Append("0"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
//
//nolint:gocyclo
func (n *Nocloud) NetworkConfiguration(ctx context.Context, st state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	// wait for devices to be ready before proceeding
	if err := netutils.WaitForDevicesReady(ctx, st); err != nil {
		return fmt.Errorf("error waiting for devices to be ready: %w", err)
	}

	metadataNetworkConfigDl, _, metadata, err := n.acquireConfig(ctx, st)
	if stderrors.Is(err, errors.ErrNoConfigSource) {
		err = nil
	}

	if err != nil {
		return err
	}

	if metadataNetworkConfigDl == nil {
		// no data, use cached network configuration if available
		return nil
	}

	unmarshalledNetworkConfig, err := DecodeNetworkConfig(metadataNetworkConfigDl)
	if err != nil {
		return err
	}

	// do a loop to retry network config remap in case of missing links
	// on each try, export the configuration as it is, and if the network is reconciled next time, export the reconciled configuration
	bckoff := backoff.NewExponentialBackOff()

	for {
		networkConfig, needsReconcile, err := n.ParseMetadata(ctx, unmarshalledNetworkConfig, st, metadata)
		if err != nil {
			return err
		}

		if !channel.SendWithContext(ctx, ch, networkConfig) {
			return ctx.Err()
		}

		if !needsReconcile {
			return nil
		}

		// wait for for backoff to retry network config remap
		nextBackoff := bckoff.NextBackOff()
		if nextBackoff == backoff.Stop {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(nextBackoff):
		}
	}
}

// DecodeNetworkConfig decodes the network configuration guessing the format from the content.
func DecodeNetworkConfig(content []byte) (*NetworkConfig, error) {
	var decoded map[string]any

	err := yaml.Unmarshal(content, &decoded)
	if err != nil {
		return nil, err
	}

	if _, ok := decoded["network"]; ok {
		var ciNetworkConfig NetworkCloudInitConfig

		err = yaml.Unmarshal(content, &ciNetworkConfig)
		if err != nil {
			return nil, err
		}

		return &ciNetworkConfig.Config, nil
	}

	// If it is not plain *v2 cloud-init* config then we attempt to decode *nocloud*
	if _, ok := decoded["version"]; ok {
		var nc NetworkConfig

		err = yaml.Unmarshal(content, &nc)
		if err != nil {
			return nil, err
		}

		return &nc, nil
	}

	return nil, fmt.Errorf("failed to decode network configuration, keys: %v", maps.Keys(decoded))
}
