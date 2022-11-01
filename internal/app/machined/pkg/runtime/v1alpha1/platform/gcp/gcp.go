// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package gcp contains the GCP implementation of the [platform.Platform].
package gcp

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"strconv"
	"strings"

	"cloud.google.com/go/compute/metadata"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// GCP is the concrete type that implements the platform.Platform interface.
type GCP struct{}

// Name implements the platform.Platform interface.
func (g *GCP) Name() string {
	return "gcp"
}

// ParseMetadata converts GCP platform metadata into platform network config.
func (g *GCP) ParseMetadata(metadata *MetadataConfig) (*runtime.PlatformNetworkConfig, error) {
	networkConfig := &runtime.PlatformNetworkConfig{}

	if metadata.Hostname != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(metadata.Hostname); err != nil {
			return nil, err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	publicIPs := []string{}

	if metadata.PublicIPv4 != "" {
		publicIPs = append(publicIPs, metadata.PublicIPv4)
	}

	dns, _ := netip.ParseAddr(gcpResolverServer) //nolint:errcheck

	networkConfig.Resolvers = append(networkConfig.Resolvers, network.ResolverSpecSpec{
		DNSServers:  []netip.Addr{dns},
		ConfigLayer: network.ConfigPlatform,
	})

	networkConfig.TimeServers = append(networkConfig.TimeServers, network.TimeServerSpecSpec{
		NTPServers:  []string{gcpTimeServer},
		ConfigLayer: network.ConfigPlatform,
	})

	region := metadata.Zone

	if idx := strings.LastIndex(region, "-"); idx != -1 {
		region = region[:idx]
	}

	for _, ipStr := range publicIPs {
		if ip, err := netip.ParseAddr(ipStr); err == nil {
			networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
		}
	}

	preempted, _ := strconv.ParseBool(metadata.Preempted) //nolint:errcheck

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:     g.Name(),
		Hostname:     metadata.Hostname,
		Region:       region,
		Zone:         metadata.Zone,
		InstanceType: metadata.InstanceType,
		InstanceID:   metadata.InstanceID,
		ProviderID:   fmt.Sprintf("gce://%s/%s/%s", metadata.ProjectID, metadata.Zone, metadata.Name),
		Spot:         preempted,
	}

	return networkConfig, nil
}

// Configuration implements the platform.Platform interface.
func (g *GCP) Configuration(ctx context.Context, r state.State) ([]byte, error) {
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
func (g *GCP) NetworkConfiguration(ctx context.Context, st state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	log.Printf("fetching gcp instance config")

	metadata, err := g.getMetadata(ctx)
	if err != nil {
		return fmt.Errorf("failed to receive GCP metadata: %w", err)
	}

	networkConfig, err := g.ParseMetadata(metadata)
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
