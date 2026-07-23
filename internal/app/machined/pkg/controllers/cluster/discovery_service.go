// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/tls"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/discovery-api/api/v1alpha1/client/pb"
	discoveryclient "github.com/siderolabs/discovery-client/pkg/client"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/siderolabs/talos/pkg/httpdefaults"
	"github.com/siderolabs/talos/pkg/machinery/client/dialer"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

const defaultDiscoveryTTL = 30 * time.Minute

// DiscoveryServiceController pushes Affiliate resources to the discovery service registry.
type DiscoveryServiceController struct {
	localAffiliateID resource.ID

	// clients holds one running discovery client per configured endpoint, keyed by endpoint name.
	clients map[string]*discoveryServiceClient
}

// discoveryServiceClientSpec captures everything that determines a client's identity and
// connection. When it changes, the client must be torn down and recreated.
type discoveryServiceClientSpec struct {
	endpoint      string // resolved host:port
	insecure      bool
	clusterID     string
	affiliateID   string
	encryptionKey []byte
}

// Equal reports whether two specs describe the same client.
func (spec discoveryServiceClientSpec) Equal(other discoveryServiceClientSpec) bool {
	return spec.endpoint == other.endpoint &&
		spec.insecure == other.insecure &&
		spec.clusterID == other.clusterID &&
		spec.affiliateID == other.affiliateID &&
		bytes.Equal(spec.encryptionKey, other.encryptionKey)
}

// discoveryServiceClient describes the state of a single running discovery client,
// one per configured discovery service endpoint.
type discoveryServiceClient struct {
	spec   discoveryServiceClientSpec
	client *discoveryclient.Client

	// per-client dedup state for SetLocalData: each client tracks what it has already received.
	prevLocalData      *pb.Affiliate
	prevLocalEndpoints []*pb.Endpoint
	prevOtherEndpoints []discoveryclient.Endpoint

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Start runs the discovery client in a goroutine, restarting it with backoff until ctx is canceled.
func (sc *discoveryServiceClient) Start(ctx context.Context, notifyCh chan<- struct{}, logger *zap.Logger, name string) {
	sc.wg.Add(1)

	ctx, sc.cancel = context.WithCancel(ctx)

	go func() {
		defer sc.wg.Done()

		sc.runWithRestarts(ctx, notifyCh, logger, name)
	}()
}

func (sc *discoveryServiceClient) runWithRestarts(ctx context.Context, notifyCh chan<- struct{}, logger *zap.Logger, name string) {
	backoff := backoff.NewExponentialBackOff()

	// disable number of retries limit
	backoff.MaxElapsedTime = 0

	for ctx.Err() == nil {
		if err := sc.runWithPanicHandler(ctx, notifyCh, logger, name); err == nil {
			// client finished without an error (clean context cancellation)
			return
		}

		interval := backoff.NextBackOff()

		logger.Debug("restarting discovery client", zap.Duration("interval", interval), zap.String("name", name))

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func (sc *discoveryServiceClient) runWithPanicHandler(ctx context.Context, notifyCh chan<- struct{}, logger *zap.Logger, name string) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)

			logger.Error("discovery client panicked", zap.Stack("stack"), zap.Error(err), zap.String("name", name))
		}
	}()

	return sc.client.Run(ctx, logger, notifyCh)
}

// Stop cancels the client and waits for its goroutine to finish.
func (sc *discoveryServiceClient) Stop() {
	sc.cancel()

	sc.wg.Wait()
}

// Name implements controller.Controller interface.
func (ctrl *DiscoveryServiceController) Name() string {
	return "cluster.DiscoveryServiceController"
}

// Inputs implements controller.Controller interface.
func (ctrl *DiscoveryServiceController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      cluster.ConfigType,
			ID:        optional.Some(cluster.ConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: cluster.NamespaceName,
			Type:      cluster.IdentityType,
			ID:        optional.Some(cluster.LocalIdentity),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: kubespan.NamespaceName,
			Type:      kubespan.EndpointType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MachineResetSignalType,
			ID:        optional.Some(runtime.MachineResetSignalID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *DiscoveryServiceController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: cluster.AffiliateType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.AddressStatusType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *DiscoveryServiceController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// shared wakeup channel for all clients: any client signals here when its discovered data changes.
	// buffered (size 1) because clients perform non-blocking sends.
	notifyCh := make(chan struct{}, 1)

	ctrl.clients = map[string]*discoveryServiceClient{}

	defer func() {
		for _, sc := range ctrl.clients {
			sc.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			if err := ctrl.reconcileClients(ctx, r, logger, notifyCh); err != nil {
				return err
			}
		case <-notifyCh:
			if err := ctrl.reconcileOutputs(ctx, r, logger); err != nil {
				return err
			}
		}

		r.ResetRestartBackoff()
	}
}

// reconcileClients brings the set of running clients in line with the configured discovery endpoints,
// then reconciles the controller outputs.
//
//nolint:gocyclo
func (ctrl *DiscoveryServiceController) reconcileClients(ctx context.Context, r controller.Runtime, logger *zap.Logger, notifyCh chan<- struct{}) error {
	clusterCfg, err := safe.ReaderGetByID[*cluster.Config](ctx, r, cluster.ConfigID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("error getting discovery config: %w", err)
	}

	clusterCfgSpec := clusterCfg.TypedSpec()

	if len(clusterCfgSpec.ServiceEndpoints) == 0 {
		// discovery is disabled: stop all clients and let the output reconcile clean up affiliates.
		ctrl.stopAllClients(logger)

		return ctrl.reconcileOutputs(ctx, r, logger)
	}

	identity, err := safe.ReaderGetByID[*cluster.Identity](ctx, r, cluster.LocalIdentity)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("error getting local identity: %w", err)
	}

	localAffiliateID := identity.TypedSpec().NodeID

	if ctrl.localAffiliateID != localAffiliateID {
		ctrl.localAffiliateID = localAffiliateID

		if err = r.UpdateInputs(append(
			ctrl.Inputs(),
			controller.Input{
				Namespace: cluster.NamespaceName,
				Type:      cluster.AffiliateType,
				ID:        optional.Some(ctrl.localAffiliateID),
				Kind:      controller.InputWeak,
			},
		)); err != nil {
			return err
		}
	}

	// figure out which clients should run, keyed by endpoint name.
	shouldRun := make(map[string]discoveryServiceClientSpec, len(clusterCfgSpec.ServiceEndpoints))

	for _, serviceEndpoint := range clusterCfgSpec.ServiceEndpoints {
		shouldRun[serviceEndpoint.Name] = discoveryServiceClientSpec{
			endpoint:      serviceEndpoint.Endpoint,
			insecure:      serviceEndpoint.Insecure,
			clusterID:     clusterCfgSpec.ServiceClusterID,
			affiliateID:   localAffiliateID,
			encryptionKey: clusterCfgSpec.ServiceEncryptionKey,
		}
	}

	// stop running clients which shouldn't run or whose spec changed. Changing the shared connection
	// parameters (encryption key, cluster ID, affiliate ID) recreates every client, as those are part
	// of each client's spec.
	for name, sc := range ctrl.clients {
		desired, exists := shouldRun[name]

		if !exists {
			logger.Info("stopping discovery client", zap.String("name", name), zap.String("endpoint", sc.spec.endpoint))

			sc.Stop()
			delete(ctrl.clients, name)
		} else if !sc.spec.Equal(desired) {
			logger.Info("recreating discovery client", zap.String("name", name), zap.String("endpoint", desired.endpoint))

			sc.Stop()
			delete(ctrl.clients, name)
		}
	}

	// start clients which aren't running.
	for name, desired := range shouldRun {
		if _, exists := ctrl.clients[name]; !exists {
			if err = ctrl.startClient(ctx, logger, notifyCh, name, desired); err != nil {
				return err
			}
		}
	}

	// now reconcile outputs as the set of clients might have changed.
	return ctrl.reconcileOutputs(ctx, r, logger)
}

// startClient builds and starts a discovery client for the given spec.
func (ctrl *DiscoveryServiceController) startClient(ctx context.Context, logger *zap.Logger, notifyCh chan<- struct{}, name string, spec discoveryServiceClientSpec) error {
	cipherBlock, err := aes.NewCipher(spec.encryptionKey)
	if err != nil {
		return fmt.Errorf("error initializing AES cipher: %w", err)
	}

	tlsConfigFunc := func() *tls.Config {
		return &tls.Config{
			RootCAs: httpdefaults.RootCAs(),
		}
	}

	client, err := discoveryclient.NewClient(discoveryclient.Options{
		Cipher:        cipherBlock,
		Endpoint:      spec.endpoint,
		ClusterID:     spec.clusterID,
		AffiliateID:   spec.affiliateID,
		TTL:           defaultDiscoveryTTL,
		Insecure:      spec.insecure,
		ClientVersion: version.Tag,
		TLSConfig:     tlsConfigFunc,
		DialOptions: []grpc.DialOption{
			grpc.WithContextDialer(dialer.DynamicProxyDialerWithTLSConfig(tlsConfigFunc)),
		},
	})
	if err != nil {
		return fmt.Errorf("error initializing discovery client %q: %w", name, err)
	}

	sc := &discoveryServiceClient{
		spec:   spec,
		client: client,
	}

	ctrl.clients[name] = sc

	logger.Info("starting discovery client", zap.String("name", name), zap.String("endpoint", spec.endpoint), zap.Bool("insecure", spec.insecure))

	sc.Start(ctx, notifyCh, logger, name)

	return nil
}

// stopAllClients stops every running client and clears the set.
func (ctrl *DiscoveryServiceController) stopAllClients(logger *zap.Logger) {
	if len(ctrl.clients) == 0 {
		return
	}

	for _, sc := range ctrl.clients {
		sc.Stop()
	}

	logger.Info("stopping all discovery clients")

	ctrl.clients = map[string]*discoveryServiceClient{}
}

// reconcileOutputs pushes the local affiliate data to every client and publishes the affiliates
// discovered across all clients, cleaning up affiliates that are no longer present.
//
//nolint:gocyclo,cyclop
func (ctrl *DiscoveryServiceController) reconcileOutputs(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if len(ctrl.clients) > 0 {
		machineResetSignal, err := safe.ReaderGetByID[*runtime.MachineResetSignal](ctx, r, runtime.MachineResetSignalID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting machine reset signal: %w", err)
		}

		affiliate, err := safe.ReaderGetByID[*cluster.Affiliate](ctx, r, ctrl.localAffiliateID)
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil
			}

			return fmt.Errorf("error getting local affiliate: %w", err)
		}

		otherEndpointsList, err := safe.ReaderListAll[*kubespan.Endpoint](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing endpoints: %w", err)
		}

		// compute the local affiliate data once; it is identical for every client.
		var (
			localData      *pb.Affiliate
			localEndpoints []*pb.Endpoint
			otherEndpoints []discoveryclient.Endpoint
		)

		if machineResetSignal == nil {
			affiliateSpec := affiliate.TypedSpec()

			localData = pbAffiliate(affiliateSpec)
			localEndpoints = affiliateToPbEndpoint(affiliateSpec)
			otherEndpoints = pbOtherEndpoints(otherEndpointsList)
		}

		for name, sc := range ctrl.clients {
			// delete/update local affiliate
			//
			// if the node enters final resetting stage, cleanup the local affiliate
			// otherwise, update local affiliate data
			if machineResetSignal != nil {
				sc.client.DeleteLocalAffiliate()

				continue
			}

			// don't send updates on localData if it hasn't changed: this introduces positive feedback loop,
			// as the watch loop will notify on self update
			if !proto.Equal(localData, sc.prevLocalData) || !equalEndpoints(localEndpoints, sc.prevLocalEndpoints) || !equalOtherEndpoints(otherEndpoints, sc.prevOtherEndpoints) {
				if err = sc.client.SetLocalData(&discoveryclient.Affiliate{
					Affiliate: localData,
					Endpoints: localEndpoints,
				}, otherEndpoints); err != nil {
					return fmt.Errorf("error setting local affiliate data on %q: %w", name, err)
				}

				sc.prevLocalData = localData
				sc.prevLocalEndpoints = localEndpoints
				sc.prevOtherEndpoints = otherEndpoints
			}
		}
	}

	r.StartTrackingOutputs()

	// discover public IP: every client observes the same public IP, use the first one reported.
	//
	// the public IP "service" AddressStatus is intentionally left out of CleanupOutputs below, so it
	// is never auto-deleted (LocalAffiliateController advertises it as a KubeSpan endpoint).
	for _, sc := range ctrl.clients {
		publicIP := sc.client.GetPublicIP()
		if len(publicIP) == 0 {
			continue
		}

		if err := safe.WriterModify(ctx, r, network.NewAddressStatus(cluster.NamespaceName, "service"), func(address *network.AddressStatus) error {
			var addr netip.Addr

			if err := addr.UnmarshalBinary(publicIP); err != nil {
				return fmt.Errorf("error unmarshaling public IP: %w", err)
			}

			address.TypedSpec().Address = netip.PrefixFrom(addr, addr.BitLen())

			return nil
		}); err != nil {
			return err
		}

		break
	}

	// discover other nodes (affiliates), aggregating across all discovery services.
	for name, sc := range ctrl.clients {
		for _, discoveredAffiliate := range sc.client.GetAffiliates() {
			id := fmt.Sprintf("service/%s", discoveredAffiliate.Affiliate.NodeId)
			logger.Debug("discovered affiliate", zap.String("id", id), zap.String("via", name))

			if err := safe.WriterModify(ctx, r, cluster.NewAffiliate(cluster.RawNamespaceName, id), func(res *cluster.Affiliate) error {
				*res.TypedSpec() = NewAffiliateSpec(discoveredAffiliate.Affiliate, discoveredAffiliate.Endpoints)

				return nil
			}); err != nil {
				return err
			}
		}
	}

	// clean up affiliates which are no longer discovered (or all of them when discovery is disabled).
	if err := r.CleanupOutputs(ctx, resource.NewMetadata(cluster.RawNamespaceName, cluster.AffiliateType, "", resource.VersionUndefined)); err != nil {
		return fmt.Errorf("error during outputs cleanup: %w", err)
	}

	return nil
}

// pbAffiliate converts a local affiliate spec into the discovery service protobuf representation.
func pbAffiliate(affiliate *cluster.AffiliateSpec) *pb.Affiliate {
	addresses := xslices.Map(affiliate.Addresses, func(address netip.Addr) []byte {
		return takeResult(address.MarshalBinary())
	})

	var kubeSpan *pb.KubeSpan

	if affiliate.KubeSpan.PublicKey != "" {
		kubeSpan = &pb.KubeSpan{
			PublicKey: affiliate.KubeSpan.PublicKey,
			Address:   takeResult(affiliate.KubeSpan.Address.MarshalBinary()),
			AdditionalAddresses: xslices.Map(affiliate.KubeSpan.AdditionalAddresses, func(address netip.Prefix) *pb.IPPrefix {
				return &pb.IPPrefix{
					Bits: uint32(address.Bits()),
					Ip:   takeResult(address.Addr().MarshalBinary()),
				}
			}),
			ExcludeAdvertisedAddresses: xslices.Map(affiliate.KubeSpan.ExcludeAdvertisedNetworks, func(address netip.Prefix) *pb.IPPrefix {
				return &pb.IPPrefix{
					Bits: uint32(address.Bits()),
					Ip:   takeResult(address.Addr().MarshalBinary()),
				}
			}),
		}
	}

	return &pb.Affiliate{
		NodeId:          affiliate.NodeID,
		Addresses:       addresses,
		Hostname:        affiliate.Hostname,
		Nodename:        affiliate.Nodename,
		MachineType:     affiliate.MachineType.String(),
		OperatingSystem: affiliate.OperatingSystem,
		Kubespan:        kubeSpan,
		ControlPlane:    controlPlaneToPb(affiliate.ControlPlane),
	}
}

// controlPlaneToPb converts cluster.ControlPlane into its protobuf representation, returning nil if absent.
func controlPlaneToPb(data *cluster.ControlPlane) *pb.ControlPlane {
	if data == nil {
		return nil
	}

	return &pb.ControlPlane{ApiServerPort: uint32(data.APIServerPort)}
}

// affiliateToPbEndpoint converts the affiliate's own KubeSpan endpoints into protobuf form, returning nil when
// KubeSpan is not configured.
func affiliateToPbEndpoint(affiliate *cluster.AffiliateSpec) []*pb.Endpoint {
	if affiliate.KubeSpan.PublicKey == "" || len(affiliate.KubeSpan.Endpoints) == 0 {
		return nil
	}

	return xslices.Map(affiliate.KubeSpan.Endpoints, func(endpoint netip.AddrPort) *pb.Endpoint {
		return &pb.Endpoint{
			Port: uint32(endpoint.Port()),
			Ip:   takeResult(endpoint.Addr().MarshalBinary()),
		}
	})
}

// pbOtherEndpoints converts KubeSpan endpoints observed for other affiliates into the discovery
// client representation, so they can be advertised on those affiliates' behalf.
func pbOtherEndpoints(otherEndpointsList safe.List[*kubespan.Endpoint]) []discoveryclient.Endpoint {
	if otherEndpointsList.Len() == 0 {
		return nil
	}

	result := make([]discoveryclient.Endpoint, 0, otherEndpointsList.Len())

	for endpoint := range otherEndpointsList.All() {
		endpointSpec := endpoint.TypedSpec()

		result = append(result, discoveryclient.Endpoint{
			AffiliateID: endpointSpec.AffiliateID,
			Endpoints: []*pb.Endpoint{
				{
					Port: uint32(endpointSpec.Endpoint.Port()),
					Ip:   takeResult(endpointSpec.Endpoint.Addr().MarshalBinary()),
				},
			},
		})
	}

	return result
}

// equalEndpoints reports whether two endpoint lists are equal (treating nil and empty as distinct).
func equalEndpoints(a, b []*pb.Endpoint) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !proto.Equal(a[i], b[i]) {
			return false
		}
	}

	return true
}

// equalOtherEndpoints reports whether two other-affiliate endpoint lists are equal (treating nil and
// empty as distinct).
func equalOtherEndpoints(a, b []discoveryclient.Endpoint) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i].AffiliateID != b[i].AffiliateID {
			return false
		}

		if !equalEndpoints(a[i].Endpoints, b[i].Endpoints) {
			return false
		}
	}

	return true
}

// NewAffiliateSpec converts a discovered affiliate (and its endpoints) from the discovery service
// protobuf representation into an AffiliateSpec.
func NewAffiliateSpec(affiliate *pb.Affiliate, endpoints []*pb.Endpoint) cluster.AffiliateSpec {
	result := cluster.AffiliateSpec{
		NodeID:          affiliate.NodeId,
		Hostname:        affiliate.Hostname,
		Nodename:        affiliate.Nodename,
		OperatingSystem: affiliate.OperatingSystem,
		MachineType:     takeResult(machine.ParseType(affiliate.MachineType)), // ignore parse error (machine.TypeUnknown)
		ControlPlane:    controlPlaneFromPb(affiliate.ControlPlane),
	}

	result.Addresses = make([]netip.Addr, 0, len(affiliate.Addresses))

	for i := range affiliate.Addresses {
		var ip netip.Addr

		if err := ip.UnmarshalBinary(affiliate.Addresses[i]); err == nil {
			result.Addresses = append(result.Addresses, ip)
		}
	}

	if affiliate.Kubespan != nil {
		result.KubeSpan.PublicKey = affiliate.Kubespan.PublicKey
		result.KubeSpan.Address.UnmarshalBinary(affiliate.Kubespan.Address) //nolint:errcheck // ignore error, address will be zero

		result.KubeSpan.AdditionalAddresses = make([]netip.Prefix, 0, len(affiliate.Kubespan.AdditionalAddresses))

		for i := range affiliate.Kubespan.AdditionalAddresses {
			var ip netip.Addr

			if err := ip.UnmarshalBinary(affiliate.Kubespan.AdditionalAddresses[i].Ip); err == nil {
				result.KubeSpan.AdditionalAddresses = append(result.KubeSpan.AdditionalAddresses, netip.PrefixFrom(ip, int(affiliate.Kubespan.AdditionalAddresses[i].Bits)))
			}
		}

		result.KubeSpan.Endpoints = make([]netip.AddrPort, 0, len(endpoints))

		for i := range endpoints {
			var ip netip.Addr

			if err := ip.UnmarshalBinary(endpoints[i].Ip); err == nil {
				result.KubeSpan.Endpoints = append(result.KubeSpan.Endpoints, netip.AddrPortFrom(ip, uint16(endpoints[i].Port)))
			}
		}

		result.KubeSpan.ExcludeAdvertisedNetworks = make([]netip.Prefix, 0, len(affiliate.Kubespan.ExcludeAdvertisedAddresses))

		for i := range affiliate.Kubespan.ExcludeAdvertisedAddresses {
			var ip netip.Addr

			if err := ip.UnmarshalBinary(affiliate.Kubespan.ExcludeAdvertisedAddresses[i].Ip); err == nil {
				result.KubeSpan.ExcludeAdvertisedNetworks = append(result.KubeSpan.ExcludeAdvertisedNetworks, netip.PrefixFrom(ip, int(affiliate.Kubespan.ExcludeAdvertisedAddresses[i].Bits)))
			}
		}
	}

	return result
}

// controlPlaneFromPb converts protobuf control plane info into the affiliate spec form, returning nil
// if absent.
func controlPlaneFromPb(plane *pb.ControlPlane) *cluster.ControlPlane {
	if plane == nil {
		return nil
	}

	return &cluster.ControlPlane{APIServerPort: int(plane.ApiServerPort)}
}

func takeResult[T any](arg1 T, _ error) T {
	return arg1
}
