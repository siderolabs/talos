// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package akamai contains the Akamai implementation of the [platform.Platform].
package akamai

import (
	"context"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	akametadata "github.com/linode/go-metadata"
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/errors"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/internal/netutils"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// Akamai is the concrete type that implements the platform.Platform interface.
type Akamai struct{}

// Name implements the platform.Platform interface.
func (a *Akamai) Name() string {
	return "akamai"
}

// ParseMetadata converts Akamai platform metadata into platform network config.
func (a *Akamai) ParseMetadata(metadata *akametadata.InstanceData, interfaceAddresses *akametadata.NetworkData) (*runtime.PlatformNetworkConfig, error) {
	networkConfig := &runtime.PlatformNetworkConfig{}

	if metadata.Label != "" {
		hostnameSpec := network.HostnameSpecSpec{
			ConfigLayer: network.ConfigPlatform,
		}

		if err := hostnameSpec.ParseFQDN(metadata.Label); err != nil {
			return nil, err
		}

		networkConfig.Hostnames = append(networkConfig.Hostnames, hostnameSpec)
	}

	publicIPs := make([]string, 0, len(interfaceAddresses.IPv4.Public)+len(interfaceAddresses.IPv6.Ranges))

	// external IP
	for _, iface := range interfaceAddresses.IPv4.Public {
		publicIPs = append(publicIPs, iface.Addr().String())
		networkConfig.Addresses = append(networkConfig.Addresses,
			network.AddressSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				LinkName:    "eth0",
				Address:     iface,
				Scope:       nethelpers.ScopeGlobal,
				Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
				Family:      nethelpers.FamilyInet4,
			},
		)
	}

	for _, iface := range interfaceAddresses.IPv4.Private {
		networkConfig.Addresses = append(networkConfig.Addresses,
			network.AddressSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				LinkName:    "eth0",
				Address:     iface,
				Scope:       nethelpers.ScopeGlobal,
				Flags:       nethelpers.AddressFlags(nethelpers.AddressPermanent),
				Family:      nethelpers.FamilyInet4,
			},
		)
	}

	for _, iface := range interfaceAddresses.IPv6.Ranges {
		publicIPs = append(publicIPs, iface.Addr().String())

		networkConfig.Addresses = append(networkConfig.Addresses,
			network.AddressSpecSpec{
				ConfigLayer: network.ConfigPlatform,
				LinkName:    "eth0",
				Address:     iface,
				Scope:       nethelpers.ScopeGlobal,
				Flags:       nethelpers.AddressFlags(nethelpers.AddressManagementTemp),
				Family:      nethelpers.FamilyInet6,
			},
		)
	}

	networkConfig.Addresses = append(networkConfig.Addresses,
		network.AddressSpecSpec{
			ConfigLayer: network.ConfigPlatform,
			LinkName:    "eth0",
			Address:     interfaceAddresses.IPv6.LinkLocal,
			Scope:       nethelpers.ScopeLink,
			Family:      nethelpers.FamilyInet6,
		},
	)

	ipv6gw, err := netip.ParseAddr(strings.Split(interfaceAddresses.IPv6.LinkLocal.String(), ":")[0] + "::1")
	if err != nil {
		return nil, err
	}

	route := network.RouteSpecSpec{
		ConfigLayer: network.ConfigPlatform,
		Gateway:     ipv6gw,
		OutLinkName: "eth0",
		Destination: interfaceAddresses.IPv6.LinkLocal,
		Table:       nethelpers.TableMain,
		Protocol:    nethelpers.ProtocolStatic,
		Type:        nethelpers.TypeUnicast,
		Family:      nethelpers.FamilyInet6,
		Priority:    1024,
	}

	route.Normalize()

	networkConfig.Routes = append(networkConfig.Routes, route)

	for _, ipStr := range publicIPs {
		if ip, err := netip.ParseAddr(ipStr); err == nil {
			networkConfig.ExternalIPs = append(networkConfig.ExternalIPs, ip)
		}
	}

	networkConfig.Metadata = &runtimeres.PlatformMetadataSpec{
		Platform:     a.Name(),
		Hostname:     metadata.Label,
		Region:       metadata.Region,
		InstanceType: metadata.Type,
		InstanceID:   strconv.Itoa(metadata.ID),
		ProviderID:   fmt.Sprintf("linode://%d", metadata.ID),
	}

	return networkConfig, nil
}

// Configuration implements the platform.Platform interface.
func (a *Akamai) Configuration(ctx context.Context, r state.State) ([]byte, error) {
	if err := netutils.Wait(ctx, r); err != nil {
		return nil, err
	}

	metadataClient, err := akametadata.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("new metadata client: %w", err)
	}

	userData, err := metadataClient.GetUserData(ctx)
	if err != nil {
		return nil, fmt.Errorf("get user data: %w", err)
	}

	if userData == "" {
		return nil, errors.ErrNoConfigSource
	}

	return []byte(userData), nil
}

// Mode implements the platform.Platform interface.
func (a *Akamai) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (a *Akamai) KernelArgs(string, quirks.Quirks) procfs.Parameters {
	return []*procfs.Parameter{
		procfs.NewParameter("console").Append("ttyS0").Append("tty0").Append("tty1"),
		procfs.NewParameter(constants.KernelParamNetIfnames).Append("0"),
	}
}

// NetworkConfiguration implements the runtime.Platform interface.
func (a *Akamai) NetworkConfiguration(ctx context.Context, r state.State, ch chan<- *runtime.PlatformNetworkConfig) error {
	// Wait for network to be ready before attempting metadata service calls
	if err := netutils.Wait(ctx, r); err != nil {
		return err
	}

	metadataClient, err := akametadata.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("new metadata client: %w", err)
	}

	metadata, err := metadataClient.GetInstance(ctx)
	if err != nil {
		return fmt.Errorf("get instance data: %w", err)
	}

	// Check if SMBIOS UUID needs to be overridden and set UUID from Linode instance ID
	// This is done here after network is ready and we have the instance metadata
	if err := a.ensureValidUUID(ctx, r, metadata.ID); err != nil {
		return fmt.Errorf("failed to ensure valid UUID: %w", err)
	}

	metadataNetworkConfig, err := metadataClient.GetNetwork(ctx)
	if err != nil {
		return fmt.Errorf("get network data: %w", err)
	}

	networkConfig, err := a.ParseMetadata(metadata, metadataNetworkConfig)
	if err != nil {
		return fmt.Errorf("parse metadata: %w", err)
	}

	select {
	case ch <- networkConfig:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

// ensureValidUUID checks if SMBIOS UUID is valid and sets override if needed.
func (a *Akamai) ensureValidUUID(ctx context.Context, r state.State, linodeID int) error {
	// First, check if there's already a UUID override set
	if uuidOverride, err := safe.ReaderGetByID[*runtimeres.MetaKey](ctx, r, runtimeres.MetaKeyTagToID(meta.UUIDOverride)); err == nil {
		if uuidOverride.TypedSpec().Value != "" {
			// UUID override already exists, don't modify it
			return nil
		}
	}

	// Check current SMBIOS UUID from SystemInformation
	systemInfo, err := safe.ReaderGetByID[*hardware.SystemInformation](ctx, r, hardware.SystemInformationID)
	if err != nil {
		// SystemInformation might not be available yet during early boot
		// In this case, we'll set the UUID override anyway as a precaution
		if !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get system information: %w", err)
		}
	}

	var shouldOverride bool
	var currentUUID string

	if systemInfo != nil {
		currentUUID = systemInfo.TypedSpec().UUID
		shouldOverride = isInvalidUUID(currentUUID)
	} else {
		// SystemInformation not available yet, assume we need override
		shouldOverride = true
		currentUUID = "not-available-yet"
	}

	if shouldOverride && linodeID > 0 {
		generatedUUID := generateLinodeUUID(linodeID)

		uuidKey := runtimeres.NewMetaKey(runtimeres.NamespaceName, runtimeres.MetaKeyTagToID(meta.UUIDOverride))
		uuidKey.TypedSpec().Value = generatedUUID

		// Try to create the UUID override key
		if err := r.Create(ctx, uuidKey); err != nil {
			if !state.IsConflictError(err) {
				return fmt.Errorf("failed to create UUID override for invalid SMBIOS UUID %q: %w", currentUUID, err)
			}
			// If it already exists (conflict), that's fine - it means UUID override is already set
		}
	}

	return nil
}

// isInvalidUUID checks if a UUID is invalid (empty or all-zeros).
func isInvalidUUID(uuid string) bool {
	if uuid == "" {
		return true
	}

	// Check for all-zeros UUID (the main issue on Linode VMs)
	if uuid == "00000000-0000-0000-0000-000000000000" {
		return true
	}

	return false
}

// generateLinodeUUID creates a UUID from Linode instance ID.
func generateLinodeUUID(linodeID int) string {
	// Create UUID format: 00000000-0000-0000-0000-{12-digit-linode-id}
	// Pad the Linode ID to 12 digits with leading zeros
	linodeIDStr := fmt.Sprintf("%012d", linodeID)
	return fmt.Sprintf("00000000-0000-0000-0000-%s", linodeIDStr)
}
