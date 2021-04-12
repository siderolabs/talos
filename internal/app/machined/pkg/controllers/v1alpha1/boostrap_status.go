// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"context"
	"fmt"
	"log"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/pkg/etcd"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/resources/v1alpha1"
)

// BootstrapStatusController manages v1alpha1.Service based on services subsystem state.
type BootstrapStatusController struct {
	V1Alpha1Events runtime.Watcher
}

// Name implements controller.Controller interface.
func (ctrl *BootstrapStatusController) Name() string {
	return "v1alpha1.BootstrapStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *BootstrapStatusController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        pointer.ToString("etcd"),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *BootstrapStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: v1alpha1.BootstrapStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *BootstrapStatusController) Run(ctx context.Context, r controller.Runtime, logger *log.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// wait for etcd to be healthy as controller reads the key
		etcdResource, err := r.Get(ctx, resource.NewMetadata(v1alpha1.NamespaceName, v1alpha1.ServiceType, "etcd", resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if !etcdResource.(*v1alpha1.Service).Healthy() {
			continue
		}

		if err = ctrl.readInitialized(ctx, r, logger); err != nil {
			return err
		}
	}
}

func (ctrl *BootstrapStatusController) readInitialized(ctx context.Context, r controller.Runtime, logger *log.Logger) error {
	etcdClient, err := etcd.NewLocalClient()
	if err != nil {
		return fmt.Errorf("error creating etcd client: %w", err)
	}

	defer etcdClient.Close() //nolint:errcheck

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ctx = clientv3.WithRequireLeader(ctx)

	// InitializedKey was created by Talos < 0.9 after successful bootstrap run (bootkube)
	// this key can only be removed by Talos 0.9, so if key is not found, controller returns

	watchCh := etcdClient.Watch(ctx, constants.InitializedKey)

	resp, err := etcdClient.Get(ctx, constants.InitializedKey)
	if err != nil {
		return fmt.Errorf("error getting key: %w", err)
	}

	if resp.Count == 0 || string(resp.Kvs[0].Value) != "true" {
		logger.Printf("bootkube initialized status not found")

		return r.Modify(ctx, v1alpha1.NewBootstrapStatus(), func(r resource.Resource) error {
			r.(*v1alpha1.BootstrapStatus).Status().SelfHostedControlPlane = false

			return nil
		})
	}

	logger.Printf("found bootkube initialized status in etcd")

	if err = r.Modify(ctx, v1alpha1.NewBootstrapStatus(), func(r resource.Resource) error {
		r.(*v1alpha1.BootstrapStatus).Status().SelfHostedControlPlane = true

		return nil
	}); err != nil {
		return err
	}

	// wait for key change or any other event in etcd
	<-watchCh

	r.QueueReconcile()

	return nil
}
