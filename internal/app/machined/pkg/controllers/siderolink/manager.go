// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
	pb "github.com/siderolabs/siderolink/api/siderolink"
	"github.com/siderolabs/siderolink/pkg/wireguard"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	networkutils "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/utils"
	"github.com/siderolabs/talos/internal/pkg/endpoint"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/siderolink"
)

// ManagerController interacts with SideroLink API and brings up the SideroLink Wireguard interface.
type ManagerController struct {
	nodeKey wgtypes.Key
}

// Name implements controller.Controller interface.
func (ctrl *ManagerController) Name() string {
	return "siderolink.ManagerController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ManagerController) Inputs() []controller.Input {
	return nil
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
//nolint:gocyclo,cyclop
func (ctrl *ManagerController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// initially, wait for the network address status to be ready
	if err := networkutils.WaitForNetworkReady(ctx, r,
		func(status *network.StatusSpec) bool {
			return status.AddressReady
		},
		[]controller.Input{
			{
				Namespace: config.NamespaceName,
				Type:      siderolink.ConfigType,
				ID:        pointer.To(siderolink.ConfigID),
				Kind:      controller.InputWeak,
			},
			{
				Namespace: hardware.NamespaceName,
				Type:      hardware.SystemInformationType,
				ID:        pointer.To(hardware.SystemInformationID),
				Kind:      controller.InputWeak,
			},
		},
	); err != nil {
		return fmt.Errorf("error waiting for network: %w", err)
	}

	// normal reconcile loop
	wgClient, wgClientErr := wgctrl.New()
	if wgClientErr != nil {
		return wgClientErr
	}

	defer func() {
		if closeErr := wgClient.Close(); closeErr != nil {
			logger.Error("failed to close wg client", zap.Error(closeErr))
		}
	}()

	var zeroKey wgtypes.Key

	if bytes.Equal(ctrl.nodeKey[:], zeroKey[:]) {
		var err error

		ctrl.nodeKey, err = wgtypes.GeneratePrivateKey()
		if err != nil {
			return fmt.Errorf("error generating Wireguard key: %w", err)
		}
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			reconnect, err := ctrl.shouldReconnect(wgClient)
			if err != nil {
				return err
			}

			if !reconnect {
				// nothing to do
				continue
			}
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGet[*siderolink.Config](ctx, r, siderolink.NewConfig(config.NamespaceName, siderolink.ConfigID).Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				if cleanupErr := ctrl.cleanup(ctx, r, nil, nil, logger); cleanupErr != nil {
					return fmt.Errorf("failed to do cleanup: %w", cleanupErr)
				}

				// no config
				continue
			}

			return fmt.Errorf("failed to get siderolink config: %w", err)
		}

		sysInfo, err := safe.ReaderGet[*hardware.SystemInformation](ctx, r, hardware.NewSystemInformation(hardware.SystemInformationID).Metadata())
		if err != nil {
			if state.IsNotFoundError(err) {
				// no system information
				continue
			}

			return fmt.Errorf("failed to get system information: %w", err)
		}

		nodeUUID := sysInfo.TypedSpec().UUID
		stringEndpoint := cfg.TypedSpec().APIEndpoint

		parsedEndpoint, err := endpoint.Parse(stringEndpoint)
		if err != nil {
			return fmt.Errorf("failed to parse siderolink endpoint: %w", err)
		}

		var transportCredentials credentials.TransportCredentials

		if parsedEndpoint.Insecure {
			transportCredentials = insecure.NewCredentials()
		} else {
			transportCredentials = credentials.NewTLS(&tls.Config{})
		}

		provision := func() (*pb.ProvisionResponse, error) {
			connCtx, connCtxCancel := context.WithTimeout(ctx, 10*time.Second)
			defer connCtxCancel()

			conn, connErr := grpc.DialContext(connCtx, parsedEndpoint.Host, grpc.WithTransportCredentials(transportCredentials))
			if connErr != nil {
				return nil, fmt.Errorf("error dialing SideroLink endpoint %q: %w", stringEndpoint, connErr)
			}

			defer func() {
				if closeErr := conn.Close(); closeErr != nil {
					logger.Error("failed to close SideroLink provisioning GRPC connection", zap.Error(closeErr))
				}
			}()

			sideroLinkClient := pb.NewProvisionServiceClient(conn)
			request := &pb.ProvisionRequest{
				NodeUuid:      nodeUUID,
				NodePublicKey: ctrl.nodeKey.PublicKey().String(),
			}

			token := parsedEndpoint.GetParam("jointoken")

			if token != "" {
				request.JoinToken = pointer.To(token)
			}

			return sideroLinkClient.Provision(ctx, request)
		}

		resp, err := provision()
		if err != nil {
			return err
		}

		serverAddress, err := netip.ParseAddr(resp.ServerAddress)
		if err != nil {
			return fmt.Errorf("error parsing server address: %w", err)
		}

		nodeAddress, err := netip.ParsePrefix(resp.NodeAddressPrefix)
		if err != nil {
			return fmt.Errorf("error parsing node address: %w", err)
		}

		linkSpec := network.NewLinkSpec(network.ConfigNamespaceName, network.LayeredID(network.ConfigOperator, network.LinkID(constants.SideroLinkName)))
		addressSpec := network.NewAddressSpec(network.ConfigNamespaceName, network.LayeredID(network.ConfigOperator, network.AddressID(constants.SideroLinkName, nodeAddress)))

		if err = safe.WriterModify(ctx, r, linkSpec,
			func(res *network.LinkSpec) error {
				spec := res.TypedSpec()

				spec.ConfigLayer = network.ConfigOperator
				spec.Name = constants.SideroLinkName
				spec.Type = nethelpers.LinkNone
				spec.Kind = "wireguard"
				spec.Up = true
				spec.Logical = true
				spec.MTU = wireguard.LinkMTU

				spec.Wireguard = network.WireguardSpec{
					PrivateKey: ctrl.nodeKey.String(),
					Peers: []network.WireguardPeer{
						{
							PublicKey: resp.ServerPublicKey,
							Endpoint:  resp.ServerEndpoint,
							AllowedIPs: []netip.Prefix{
								netip.PrefixFrom(serverAddress, serverAddress.BitLen()),
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

		if err = safe.WriterModify(ctx, r, addressSpec,
			func(res *network.AddressSpec) error {
				spec := res.TypedSpec()

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

		keepLinkSpecSet := map[resource.ID]struct{}{
			linkSpec.Metadata().ID(): {},
		}

		keepAddressSpecSet := map[resource.ID]struct{}{
			addressSpec.Metadata().ID(): {},
		}

		if err = ctrl.cleanup(ctx, r, keepLinkSpecSet, keepAddressSpecSet, logger); err != nil {
			return err
		}

		logger.Info(
			"siderolink connection configured",
			zap.String("endpoint", stringEndpoint),
			zap.String("node_uuid", nodeUUID),
			zap.String("node_address", nodeAddress.String()),
		)
	}
}

func (ctrl *ManagerController) cleanup(
	ctx context.Context,
	r controller.Runtime,
	keepLinkSpecIDSet, keepAddressSpecIDSet map[resource.ID]struct{},
	logger *zap.Logger,
) error {
	if err := ctrl.cleanupLinkSpecs(ctx, r, keepLinkSpecIDSet, logger); err != nil {
		return err
	}

	return ctrl.cleanupAddressSpecs(ctx, r, keepAddressSpecIDSet, logger)
}

//nolint:dupl
func (ctrl *ManagerController) cleanupLinkSpecs(ctx context.Context, r controller.Runtime, keepSet map[resource.ID]struct{}, logger *zap.Logger) error {
	list, err := safe.ReaderList[*network.LinkSpec](ctx, r, network.NewLinkSpec(network.ConfigNamespaceName, "").Metadata())
	if err != nil {
		return err
	}

	for iter := safe.IteratorFromList(list); iter.Next(); {
		link := iter.Value()

		if link.Metadata().Owner() != ctrl.Name() {
			continue
		}

		if _, ok := keepSet[link.Metadata().ID()]; ok {
			continue
		}

		if destroyErr := r.Destroy(ctx, link.Metadata()); destroyErr != nil && !state.IsNotFoundError(destroyErr) {
			return destroyErr
		}

		logger.Info("destroyed link spec", zap.String("link_id", link.Metadata().ID()))
	}

	return nil
}

//nolint:dupl
func (ctrl *ManagerController) cleanupAddressSpecs(ctx context.Context, r controller.Runtime, keepSet map[resource.ID]struct{}, logger *zap.Logger) error {
	list, err := safe.ReaderList[*network.AddressSpec](ctx, r, network.NewAddressSpec(network.ConfigNamespaceName, "").Metadata())
	if err != nil {
		return err
	}

	for iter := safe.IteratorFromList(list); iter.Next(); {
		address := iter.Value()

		if address.Metadata().Owner() != ctrl.Name() {
			continue
		}

		if _, ok := keepSet[address.Metadata().ID()]; ok {
			continue
		}

		if destroyErr := r.Destroy(ctx, address.Metadata()); destroyErr != nil && !state.IsNotFoundError(destroyErr) {
			return destroyErr
		}

		logger.Info("destroyed address spec", zap.String("address_id", address.Metadata().ID()))
	}

	return nil
}

func (ctrl *ManagerController) shouldReconnect(wgClient *wgctrl.Client) (bool, error) {
	wgDevice, err := wgClient.Device(constants.SideroLinkName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// no Wireguard device, so no need to reconnect
			return false, nil
		}

		return false, fmt.Errorf("error reading Wireguard device: %w", err)
	}

	if len(wgDevice.Peers) != 1 {
		return false, fmt.Errorf("unexpected number of Wireguard peers: %d", len(wgDevice.Peers))
	}

	peer := wgDevice.Peers[0]
	since := time.Since(peer.LastHandshakeTime)

	return since >= wireguard.PeerDownInterval, nil
}
