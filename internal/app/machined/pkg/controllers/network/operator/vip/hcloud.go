// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vip

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"

	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/download"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// HCloudHandler implements assignment and release of Virtual IPs using API.
type HCloudHandler struct {
	client *hcloud.Client

	logger *zap.Logger

	vip        string
	deviceID   int64
	floatingID int64
	networkID  int64
}

// NewHCloudHandler creates new NewEHCloudHandler.
func NewHCloudHandler(logger *zap.Logger, vip string, spec network.VIPHCloudSpec) *HCloudHandler {
	return &HCloudHandler{
		client: hcloud.NewClient(hcloud.WithToken(spec.APIToken)),

		logger: logger,

		vip:       vip,
		deviceID:  spec.DeviceID,
		networkID: spec.NetworkID,
	}
}

// Acquire implements Handler interface.
func (handler *HCloudHandler) Acquire(ctx context.Context) error {
	if handler.networkID > 0 {
		var action *hcloud.Action

		alias := hcloud.ServerChangeAliasIPsOpts{
			Network:  &hcloud.Network{ID: handler.networkID},
			AliasIPs: []net.IP{},
		}

		// trying to find the old active server
		// and remove alias IP from it
		serverList, err := handler.client.Server.All(ctx)
		if err != nil {
			return fmt.Errorf("error getting server list: %w", err)
		}

		oldDeviceID := findServerByAlias(serverList, handler.networkID, handler.vip)
		if oldDeviceID != 0 {
			handler.logger.Info("trying to remove previous Hetzner Cloud IP alias",
				zap.String("vip", handler.vip), zap.Int64("device_id", oldDeviceID),
				zap.Int64("network_id", handler.networkID),
			)

			action, _, err = handler.client.Server.ChangeAliasIPs(ctx,
				&hcloud.Server{ID: oldDeviceID},
				hcloud.ServerChangeAliasIPsOpts{
					Network:  &hcloud.Network{ID: handler.networkID},
					AliasIPs: []net.IP{},
				},
			)
			if err != nil {
				return fmt.Errorf("error remove Hetzner Cloud IP alias %q from server %d: %w", handler.vip, oldDeviceID, err)
			}

			handler.logger.Info("cleared previous Hetzner Cloud IP alias", zap.String("vip", handler.vip),
				zap.Int64("device_id", oldDeviceID), zap.String("status", string(action.Status)))
		}

		netIP := net.ParseIP(handler.vip)
		alias.AliasIPs = []net.IP{netIP}

		action, _, err = handler.client.Server.ChangeAliasIPs(ctx,
			&hcloud.Server{ID: handler.deviceID},
			alias)
		if err != nil {
			return fmt.Errorf("error change alias IPs %q to server %d: %w", handler.vip, handler.deviceID, err)
		}

		handler.logger.Info("assigned Hetzner Cloud IP alias", zap.String("vip", handler.vip), zap.Int64("device_id", handler.deviceID),
			zap.Int64("network_id", handler.networkID), zap.String("status", string(action.Status)))

		return nil
	}

	floatips, err := handler.client.FloatingIP.All(ctx)
	if err != nil {
		return fmt.Errorf("error getting floatingIPs list: %w", err)
	}

	for _, floatip := range floatips {
		if floatip.IP.String() == handler.vip {
			action, _, err := handler.client.FloatingIP.Assign(ctx, floatip, &hcloud.Server{ID: handler.deviceID})
			if err != nil {
				return fmt.Errorf("error assigning %q on server %d: %w", handler.vip, handler.deviceID, err)
			}

			handler.logger.Info("assigned Hetzner Cloud floating IP", zap.String("vip", handler.vip), zap.Int64("device_id", handler.deviceID), zap.String("status", string(action.Status)))
			handler.floatingID = floatip.ID

			return nil
		}
	}

	return fmt.Errorf("error assigning %q to server %d in network %d: floating IP / alias IP is not found", handler.vip, handler.deviceID, handler.networkID)
}

// Release implements Handler interface.
func (handler *HCloudHandler) Release(ctx context.Context) error {
	if handler.networkID > 0 {
		alias := hcloud.ServerChangeAliasIPsOpts{
			Network:  &hcloud.Network{ID: handler.networkID},
			AliasIPs: []net.IP{},
		}

		action, _, err := handler.client.Server.ChangeAliasIPs(ctx,
			&hcloud.Server{ID: handler.deviceID},
			alias)
		if err != nil {
			return fmt.Errorf("error remove alias IPs %q on server %d: %w", handler.vip, handler.deviceID, err)
		}

		handler.logger.Info("unassigned Hetzner Cloud alias IP", zap.String("vip", handler.vip), zap.Int64("device_id", handler.deviceID),
			zap.Int64("network_id", handler.networkID), zap.String("status", string(action.Status)))

		return nil
	}

	if handler.floatingID > 0 {
		floatip, _, err := handler.client.FloatingIP.GetByID(ctx, handler.floatingID)
		if err != nil {
			return fmt.Errorf("error getting floatingIP info: %w", err)
		}

		if floatip.Server == nil || floatip.Server.ID != handler.deviceID {
			handler.logger.Info("unassigned Hetzner Cloud floating IP", zap.String("vip", handler.vip), zap.Int64("device_id", handler.deviceID))
		}

		handler.floatingID = 0
	}

	return nil
}

// HCloudMetaDataEndpoint is the local endpoint for machine info like networking.
const HCloudMetaDataEndpoint = "http://169.254.169.254/hetzner/v1/metadata/instance-id"

// GetNetworkAndDeviceIDs fills in parts of the spec based on the API token and instance metadata.
func GetNetworkAndDeviceIDs(ctx context.Context, spec *network.VIPHCloudSpec, vip netip.Addr, logger *zap.Logger) error {
	metadataInstanceID, err := download.Download(ctx, HCloudMetaDataEndpoint)
	if err != nil {
		return fmt.Errorf("error downloading instance-id: %w", err)
	}

	spec.DeviceID, err = strconv.ParseInt(string(metadataInstanceID), 10, 64)
	if err != nil {
		return fmt.Errorf("error getting instance-id id: %w", err)
	}

	client := hcloud.NewClient(hcloud.WithToken(spec.APIToken))

	server, _, err := client.Server.GetByID(ctx, spec.DeviceID)
	if err != nil {
		return fmt.Errorf("error getting server info: %w", err)
	}

	if vip.IsPrivate() {
		// find private network for private vip (alias IP)
		assignedPrivateNetworks := server.PrivateNet
		if len(assignedPrivateNetworks) == 0 {
			return fmt.Errorf("trying to assign private vip (alias IP) %q, but no private network is assigned to the server", vip.String())
		}

		for _, assignedPrivateNetwork := range assignedPrivateNetworks {
			privateNetwork, _, err := client.Network.GetByID(ctx, assignedPrivateNetwork.Network.ID)
			if err != nil {
				return fmt.Errorf("error getting network info: %w", err)
			}

			if privateNetwork.IPRange.Contains(vip.AsSlice()) {
				spec.NetworkID = assignedPrivateNetwork.Network.ID
				logger.Info("found private network for private vip (alias IP)", zap.String("vip", vip.String()), zap.Int64("network_id", spec.NetworkID))

				break
			}
		}
	} else {
		// the public vip (floating IP) doesn't require a private network
		spec.NetworkID = 0
	}

	return nil
}

func findServerByAlias(serverList []*hcloud.Server, networkID int64, vip string) (deviceID int64) {
	for _, server := range serverList {
		for _, network := range server.PrivateNet {
			if network.Network.ID == networkID {
				for _, alias := range network.Aliases {
					if alias.String() == vip {
						return server.ID
					}
				}
			}
		}
	}

	return 0
}
