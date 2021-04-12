// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"context"
	"fmt"
	"log"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/talos-systems/talos/internal/pkg/etcd"
	"github.com/talos-systems/talos/pkg/resources/secrets"
	"github.com/talos-systems/talos/pkg/resources/time"
	"github.com/talos-systems/talos/pkg/resources/v1alpha1"
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
			Type:      secrets.RootType,
			ID:        pointer.ToString(secrets.RootEtcdID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        pointer.ToString("networkd"),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      time.StatusType,
			ID:        pointer.ToString(time.StatusID),
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
func (ctrl *EtcdController) Run(ctx context.Context, r controller.Runtime, logger *log.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		etcdRootRes, err := r.Get(ctx, resource.NewMetadata(secrets.NamespaceName, secrets.RootType, secrets.RootEtcdID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				if err = ctrl.teardownAll(ctx, r); err != nil {
					return fmt.Errorf("error destroying resources: %w", err)
				}

				continue
			}

			return fmt.Errorf("error getting etcd root secrets: %w", err)
		}

		etcdRoot := etcdRootRes.(*secrets.Root).EtcdSpec()

		// wait for networkd to be healthy as it might change IPs/hostname
		networkdResource, err := r.Get(ctx, resource.NewMetadata(v1alpha1.NamespaceName, v1alpha1.ServiceType, "networkd", resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if !networkdResource.(*v1alpha1.Service).Healthy() {
			continue
		}

		// wait for time sync as certs depend on current time
		timeSyncResource, err := r.Get(ctx, resource.NewMetadata(v1alpha1.NamespaceName, time.StatusType, time.StatusID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if !timeSyncResource.(*time.Status).Status().Synced {
			continue
		}

		if err = r.Modify(ctx, secrets.NewEtcd(), func(r resource.Resource) error {
			return ctrl.updateSecrets(etcdRoot, r.(*secrets.Etcd).Certs())
		}); err != nil {
			return err
		}
	}
}

func (ctrl *EtcdController) updateSecrets(etcdRoot *secrets.RootEtcdSpec, etcdCerts *secrets.EtcdCertsSpec) error {
	var err error

	etcdCerts.EtcdPeer, err = etcd.GeneratePeerCert(etcdRoot.EtcdCA)
	if err != nil {
		return fmt.Errorf("error generating etcd certs: %w", err)
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
