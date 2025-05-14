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
	"github.com/siderolabs/gen/optional"
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
	"github.com/siderolabs/talos/pkg/grpc/dialer"
	"github.com/siderolabs/talos/pkg/httpdefaults"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// ManagerController interacts with SideroLink API and brings up the SideroLink Wireguard interface.
type ManagerController struct {
	nodeKey wgtypes.Key
	pd      provisionData
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
		{
			Type: siderolink.TunnelType,
			Kind: controller.OutputExclusive,
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
				ID:        optional.Some(siderolink.ConfigID),
				Kind:      controller.InputWeak,
			},
			{
				Namespace: hardware.NamespaceName,
				Type:      hardware.SystemInformationType,
				ID:        optional.Some(hardware.SystemInformationID),
				Kind:      controller.InputWeak,
			},
			{
				Namespace: runtime.NamespaceName,
				Type:      runtime.UniqueMachineTokenType,
				ID:        optional.Some(runtime.UniqueMachineTokenID),
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

	// default name, actual name is set based on the provision API response:
	// whether we use the Wireguard tunnel over gRPC or not
	linkName := constants.SideroLinkName

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			reconnect, err := peerDown(wgClient, linkName)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					// no Wireguard device, so no need to reconnect
					continue
				}

				return err
			}

			if !reconnect {
				// nothing to do
				continue
			}
		case <-r.EventCh():
		}

		if ctrl.pd.IsEmpty() {
			provision, err := ctrl.provision(ctx, r, logger)
			if err != nil {
				return fmt.Errorf("error provisioning: %w", err)
			}

			if !provision.IsPresent() {
				continue
			}

			ctrl.pd = provision.ValueOrZero()
		}

		useWgTunnel := ctrl.pd.grpcPeerAddrPort != ""

		if useWgTunnel {
			linkName = constants.SideroLinkTunnelName
		} else {
			linkName = constants.SideroLinkName
		}

		serverAddress, err := netip.ParseAddr(ctrl.pd.ServerAddress)
		if err != nil {
			return fmt.Errorf("error parsing server address: %w", err)
		}

		nodeAddress, err := netip.ParsePrefix(ctrl.pd.NodeAddressPrefix)
		if err != nil {
			return fmt.Errorf("error parsing node address: %w", err)
		}

		linkSpec := network.NewLinkSpec(network.ConfigNamespaceName, network.LayeredID(network.ConfigOperator, network.LinkID(linkName)))
		addressSpec := network.NewAddressSpec(network.ConfigNamespaceName, network.LayeredID(network.ConfigOperator, network.AddressID(linkName, nodeAddress)))

		// Rotate through the endpoints.
		ep, ok := ctrl.pd.TakeEndpoint()
		if !ok {
			return errors.New("host returned no endpoints")
		}

		logger.Info(
			"configuring siderolink connection",
			zap.String("peer_endpoint", ep),
			zap.String("next_peer_endpoint", ctrl.pd.PeekNextEndpoint()),
		)

		if err = safe.WriterModify(ctx, r, linkSpec,
			func(res *network.LinkSpec) error {
				spec := res.TypedSpec()

				spec.ConfigLayer = network.ConfigOperator
				spec.Name = linkName
				spec.Type = nethelpers.LinkNone
				spec.Kind = "wireguard"

				// if using wg-tunnel, the actual link will be created in the userspace
				// as a tunnel device
				// if using native, we create a native kernel wireguard interface
				if useWgTunnel {
					spec.Logical = false // the controller does not create the link
				} else {
					spec.Logical = true // allow controller to create the link
				}

				spec.Up = true
				spec.MTU = wireguard.LinkMTU

				spec.Wireguard = network.WireguardSpec{
					PrivateKey: ctrl.nodeKey.String(),
					Peers: []network.WireguardPeer{
						{
							PublicKey: ctrl.pd.ServerPublicKey,
							Endpoint:  ep,
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
				spec.LinkName = linkName
				spec.Scope = nethelpers.ScopeGlobal

				return nil
			}); err != nil {
			return fmt.Errorf("error creating address spec: %w", err)
		}

		if ctrl.pd.grpcPeerAddrPort != "" {
			var ourAddr netip.AddrPort

			ourAddr, err = netip.ParseAddrPort(ctrl.pd.grpcPeerAddrPort)
			if err != nil {
				return err
			}

			if err = safe.WriterModify(ctx, r, siderolink.NewTunnel(),
				func(tunnel *siderolink.Tunnel) error {
					tunnel.TypedSpec().APIEndpoint = ctrl.pd.apiEndpont
					tunnel.TypedSpec().LinkName = linkName
					tunnel.TypedSpec().MTU = wireguard.LinkMTU
					tunnel.TypedSpec().NodeAddress = ourAddr

					return nil
				},
			); err != nil {
				return fmt.Errorf("error creating tunnel spec: %w", err)
			}
		} else {
			if err = r.Destroy(ctx, siderolink.NewTunnel().Metadata()); err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error destroying tunnel spec: %w", err)
			}
		}

		keepLinkSpecSet := map[resource.ID]struct{}{
			linkSpec.Metadata().ID(): {},
		}

		keepAddressSpecSet := map[resource.ID]struct{}{
			addressSpec.Metadata().ID(): {},
		}

		if err := ctrl.cleanup(ctx, r, keepLinkSpecSet, keepAddressSpecSet, logger); err != nil {
			return err
		}

		logger.Info(
			"siderolink connection configured",
			zap.String("endpoint", ctrl.pd.apiEndpont),
			zap.String("node_uuid", ctrl.pd.nodeUUID),
			zap.String("node_address", nodeAddress.String()),
		)
	}
}

//nolint:gocyclo
func (ctrl *ManagerController) provision(ctx context.Context, r controller.Runtime, logger *zap.Logger) (optional.Optional[provisionData], error) {
	cfg, err := safe.ReaderGetByID[*siderolink.Config](ctx, r, siderolink.ConfigID)
	if err != nil {
		if state.IsNotFoundError(err) {
			if cleanupErr := ctrl.cleanup(ctx, r, nil, nil, logger); cleanupErr != nil {
				return optional.None[provisionData](), fmt.Errorf("failed to do cleanup: %w", cleanupErr)
			}

			// no config
			return optional.None[provisionData](), nil
		}

		return optional.None[provisionData](), fmt.Errorf("failed to get siderolink config: %w", err)
	}

	sysInfo, err := safe.ReaderGetByID[*hardware.SystemInformation](ctx, r, hardware.SystemInformationID)
	if err != nil {
		if state.IsNotFoundError(err) {
			// no system information
			return optional.None[provisionData](), nil
		}

		return optional.None[provisionData](), fmt.Errorf("failed to get system information: %w", err)
	}

	nodeUUID := sysInfo.TypedSpec().UUID

	provision := func() (*pb.ProvisionResponse, error) {
		conn, connErr := grpc.NewClient(
			cfg.TypedSpec().Host,
			withTransportCredentials(cfg.TypedSpec().Insecure),
			grpc.WithSharedWriteBuffer(true),
			grpc.WithContextDialer(dialer.DynamicProxyDialer),
		)
		if connErr != nil {
			return nil, fmt.Errorf("error dialing SideroLink endpoint %q: %w", cfg.TypedSpec().Host, connErr)
		}

		defer func() {
			if closeErr := conn.Close(); closeErr != nil {
				logger.Error("failed to close SideroLink provisioning GRPC connection", zap.Error(closeErr))
			}
		}()

		uniqTokenRes, rdrErr := safe.ReaderGetByID[*runtime.UniqueMachineToken](ctx, r, runtime.UniqueMachineTokenID)
		if rdrErr != nil {
			return nil, fmt.Errorf("failed to get unique token: %w", rdrErr)
		}

		var wgOverGRPC *bool

		if cfg.TypedSpec().Tunnel {
			wgOverGRPC = pointer.To(true)
		}

		sideroLinkClient := pb.NewProvisionServiceClient(conn)
		request := &pb.ProvisionRequest{
			NodeUuid:          nodeUUID,
			NodePublicKey:     ctrl.nodeKey.PublicKey().String(),
			NodeUniqueToken:   pointer.To(uniqTokenRes.TypedSpec().Token),
			TalosVersion:      pointer.To(version.Tag),
			WireguardOverGrpc: wgOverGRPC,
		}

		token := cfg.TypedSpec().JoinToken

		if token != "" {
			request.JoinToken = pointer.To(token)
		}

		return sideroLinkClient.Provision(ctx, request)
	}

	resp, err := provision()
	if err != nil {
		return optional.None[provisionData](), err
	}

	return optional.Some(provisionData{
		nodeUUID:          nodeUUID,
		apiEndpont:        cfg.TypedSpec().APIEndpoint,
		ServerAddress:     resp.ServerAddress,
		ServerPublicKey:   resp.ServerPublicKey,
		NodeAddressPrefix: resp.NodeAddressPrefix,
		endpoints:         resp.GetEndpoints(),
		grpcPeerAddrPort:  resp.GrpcPeerAddrPort,
	}), nil
}

type provisionData struct {
	nodeUUID          string
	apiEndpont        string
	ServerAddress     string
	ServerPublicKey   string
	NodeAddressPrefix string
	endpoints         []string
	grpcPeerAddrPort  string
}

func (d *provisionData) IsEmpty() bool {
	return d == nil || len(d.endpoints) == 0
}

func (d *provisionData) TakeEndpoint() (string, bool) {
	if d.IsEmpty() {
		return "", false
	}

	ep := d.endpoints[0]
	d.endpoints = d.endpoints[1:]

	return ep, true
}

func (d *provisionData) PeekNextEndpoint() string {
	if d.IsEmpty() {
		return ""
	}

	return d.endpoints[0]
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

	for link := range list.All() {
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

	for address := range list.All() {
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

func withTransportCredentials(insec bool) grpc.DialOption {
	var transportCredentials credentials.TransportCredentials

	if insec {
		transportCredentials = insecure.NewCredentials()
	} else {
		transportCredentials = credentials.NewTLS(&tls.Config{
			RootCAs: httpdefaults.RootCAs(),
		})
	}

	return grpc.WithTransportCredentials(transportCredentials)
}
