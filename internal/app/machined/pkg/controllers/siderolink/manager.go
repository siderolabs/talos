// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink

import (
	"bytes"
	"context"
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/talos-systems/go-procfs/procfs"
	"github.com/talos-systems/go-smbios/smbios"
	pb "github.com/talos-systems/siderolink/api/siderolink"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// ManagerController interacts with SideroLink API and brings up the SideroLink Wireguard interface.
type ManagerController struct {
	Cmdline *procfs.Cmdline

	nodeKey wgtypes.Key
}

// Name implements controller.Controller interface.
func (ctrl *ManagerController) Name() string {
	return "siderolink.ManagerController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ManagerController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.StatusType,
			ID:        pointer.ToString(network.StatusID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ManagerController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.AddressSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.LinkSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *ManagerController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.Cmdline == nil || ctrl.Cmdline.Get(constants.KernelParamSideroLink).First() == nil {
		// no SideroLink command line argument, skip controller
		return nil
	}

	s, err := smbios.New()
	if err != nil {
		return fmt.Errorf("error reading node UUID: %w", err)
	}

	nodeUUID, err := s.SystemInformation().UUID()
	if err != nil {
		return fmt.Errorf("error getting node UUID: %w", err)
	}

	var zeroKey wgtypes.Key

	if bytes.Equal(ctrl.nodeKey[:], zeroKey[:]) {
		ctrl.nodeKey, err = wgtypes.GeneratePrivateKey()
		if err != nil {
			return fmt.Errorf("error generating Wireguard key: %w", err)
		}
	}

	apiEndpoint := *ctrl.Cmdline.Get(constants.KernelParamSideroLink).First()

	conn, err := grpc.DialContext(ctx, apiEndpoint, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("error dialing SideroLink endpoint %q: %w", apiEndpoint, err)
	}

	sideroLinkClient := pb.NewProvisionServiceClient(conn)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		netStatus, err := r.Get(ctx, resource.NewMetadata(network.NamespaceName, network.StatusType, network.StatusID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				// no network state yet
				continue
			}

			return fmt.Errorf("error reading network status: %w", err)
		}

		if !netStatus.(*network.Status).TypedSpec().AddressReady {
			// wait for address
			continue
		}

		resp, err := sideroLinkClient.Provision(ctx, &pb.ProvisionRequest{
			NodeUuid:      nodeUUID.String(),
			NodePublicKey: ctrl.nodeKey.PublicKey().String(),
		})
		if err != nil {
			return fmt.Errorf("error accessing SideroLink API: %w", err)
		}

		serverAddress, err := netaddr.ParseIP(resp.ServerAddress)
		if err != nil {
			return fmt.Errorf("error parsing server address: %w", err)
		}

		nodeAddress, err := netaddr.ParseIPPrefix(resp.NodeAddressPrefix)
		if err != nil {
			return fmt.Errorf("error parsing node address: %w", err)
		}

		if err = r.Modify(ctx,
			network.NewLinkSpec(network.ConfigNamespaceName, network.LayeredID(network.ConfigOperator, network.LinkID(constants.SideroLinkName))),
			func(r resource.Resource) error {
				spec := r.(*network.LinkSpec).TypedSpec()

				spec.ConfigLayer = network.ConfigOperator
				spec.Name = constants.SideroLinkName
				spec.Type = nethelpers.LinkNone
				spec.Kind = "wireguard"
				spec.Up = true
				spec.Logical = true

				spec.Wireguard = network.WireguardSpec{
					PrivateKey: ctrl.nodeKey.String(),
					Peers: []network.WireguardPeer{
						{
							PublicKey: resp.ServerPublicKey,
							Endpoint:  resp.ServerEndpoint,
							AllowedIPs: []netaddr.IPPrefix{
								netaddr.IPPrefixFrom(serverAddress, serverAddress.BitLen()),
							},
							// make sure Talos pings SideroLink endpoint, so that tunnel is established:
							// SideroLink doesn't know Talos endpoint.
							PersistentKeepaliveInterval: constants.SideroLinkDefaultPeerKeepalive,
						},
					},
				}
				spec.Wireguard.Sort()

				return nil
			}); err != nil {
			return fmt.Errorf("error creating siderolink spec: %w", err)
		}

		if err = r.Modify(ctx,
			network.NewAddressSpec(network.ConfigNamespaceName, network.LayeredID(network.ConfigOperator, network.AddressID(constants.SideroLinkName, nodeAddress))),
			func(r resource.Resource) error {
				spec := r.(*network.AddressSpec).TypedSpec()

				spec.ConfigLayer = network.ConfigOperator
				spec.Address = nodeAddress
				spec.Family = nethelpers.FamilyInet6
				spec.Flags = nethelpers.AddressFlags(nethelpers.AddressPermanent)
				spec.LinkName = constants.SideroLinkName
				spec.Scope = nethelpers.ScopeGlobal

				return nil
			}); err != nil {
			return fmt.Errorf("error creating address spec: %w", err)
		}

		// all done, terminate controller
		return nil
	}
}
