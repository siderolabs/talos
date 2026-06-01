// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	"github.com/siderolabs/talos/pkg/machinery/resources/time"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// EtcdController manages secrets.Etcd based on configuration.
type EtcdController struct{}

// Name implements controller.Controller interface.
func (ctrl *EtcdController) Name() string {
	return "secrets.EtcdController"
}

// Inputs implements controller.Controller interface.
func (ctrl *EtcdController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.EtcdRootType,
			ID:        optional.Some(secrets.EtcdRootID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.StatusType,
			ID:        optional.Some(network.StatusID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      time.StatusType,
			ID:        optional.Some(time.StatusID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.HostnameStatusType,
			ID:        optional.Some(network.HostnameID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressType,
			ID:        optional.Some(network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, k8s.NodeAddressFilterNoK8s)),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *EtcdController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: secrets.EtcdType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *EtcdController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		etcdRootRes, err := safe.ReaderGet[*secrets.EtcdRoot](ctx, r, resource.NewMetadata(secrets.NamespaceName, secrets.EtcdRootType, secrets.EtcdRootID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardownAll(ctx, r); err != nil {
					return fmt.Errorf("error destroying resources: %w", err)
				}

				continue
			}

			return fmt.Errorf("error getting etcd root secrets: %w", err)
		}

		etcdRoot := etcdRootRes.TypedSpec()

		// wait for network to be ready as it might change IPs/hostname
		networkResource, err := safe.ReaderGet[*network.Status](ctx, r, resource.NewMetadata(network.NamespaceName, network.StatusType, network.StatusID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		networkStatus := networkResource.TypedSpec()

		if !(networkStatus.AddressReady && networkStatus.HostnameReady) {
			continue
		}

		// wait for time sync as certs depend on current time
		timeSyncResource, err := safe.ReaderGet[*time.Status](ctx, r, resource.NewMetadata(v1alpha1.NamespaceName, time.StatusType, time.StatusID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if !timeSyncResource.TypedSpec().Synced {
			continue
		}

		hostnameStatus, err := safe.ReaderGet[*network.HostnameStatus](ctx, r, resource.NewMetadata(network.NamespaceName, network.HostnameStatusType, network.HostnameID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting hostname status: %w", err)
		}

		nodeAddrs, err := safe.ReaderGet[*network.NodeAddress](
			ctx,
			r,
			resource.NewMetadata(
				network.NamespaceName,
				network.NodeAddressType,
				network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, k8s.NodeAddressFilterNoK8s),
				resource.VersionUndefined,
			),
		)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting addresses: %w", err)
		}

		if err = safe.WriterModify(ctx, r, secrets.NewEtcd(), func(r *secrets.Etcd) error {
			return ctrl.updateSecrets(etcdRoot, nodeAddrs, hostnameStatus, r.TypedSpec())
		}); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *EtcdController) updateSecrets(etcdRoot *secrets.EtcdRootSpec, nodeAddress *network.NodeAddress, hostnameStatus *network.HostnameStatus, etcdCerts *secrets.EtcdCertsSpec) error {
	generator := etcd.CertificateGenerator{
		CA: etcdRoot.EtcdCA,

		NodeAddresses:  nodeAddress,
		HostnameStatus: hostnameStatus,
	}

	var err error

	etcdCerts.Etcd, err = generator.GenerateServerCert()
	if err != nil {
		return fmt.Errorf("error generating etcd client certs: %w", err)
	}

	etcdCerts.EtcdPeer, err = generator.GeneratePeerCert()
	if err != nil {
		return fmt.Errorf("error generating etcd peer certs: %w", err)
	}

	etcdCerts.EtcdAdmin, err = generator.GenerateClientCert("talos")
	if err != nil {
		return fmt.Errorf("error generating admin client certs: %w", err)
	}

	etcdCerts.EtcdAPIServer, err = generator.GenerateClientCert("kube-apiserver")
	if err != nil {
		return fmt.Errorf("error generating kube-apiserver etcd client certs: %w", err)
	}

	return nil
}

func (ctrl *EtcdController) teardownAll(ctx context.Context, r controller.Runtime) error {
	list, err := r.List(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.EtcdType, "", resource.VersionUndefined))
	if err != nil {
		return err
	}

	// TODO: change this to proper teardown sequence

	for _, res := range list.Items {
		if err = r.Destroy(ctx, res.Metadata()); err != nil {
			return err
		}
	}

	return nil
}
