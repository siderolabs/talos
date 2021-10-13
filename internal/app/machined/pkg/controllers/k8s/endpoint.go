// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"inet.af/netaddr"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/talos-systems/talos/pkg/conditions"
	"github.com/talos-systems/talos/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/resources/config"
	"github.com/talos-systems/talos/pkg/resources/k8s"
)

// EndpointController looks up control plane endpoints.
type EndpointController struct{}

// Name implements controller.Controller interface.
func (ctrl *EndpointController) Name() string {
	return "k8s.EndpointController"
}

// Inputs implements controller.Controller interface.
func (ctrl *EndpointController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        pointer.ToString(config.MachineTypeID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *EndpointController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.EndpointType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *EndpointController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		machineTypeRes, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineTypeType, config.MachineTypeID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting machine type: %w", err)
		}

		machineType := machineTypeRes.(*config.MachineType).MachineType()

		if machineType != machine.TypeWorker {
			// TODO: implemented only for machine.TypeWorker for now, should be extended to support control plane machines (for etcd join).
			continue
		}

		logger.Debug("waiting for kubelet client config", zap.String("file", constants.KubeletKubeconfig))

		if err = conditions.WaitForKubeconfigReady(constants.KubeletKubeconfig).Wait(ctx); err != nil {
			return err
		}

		if err = ctrl.watchEndpoints(ctx, r, logger); err != nil {
			return err
		}
	}
}

//nolint:gocyclo
func (ctrl *EndpointController) watchEndpoints(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	client, err := kubernetes.NewClientFromKubeletKubeconfig()
	if err != nil {
		return fmt.Errorf("error building Kubernetes client: %w", err)
	}

	defer client.Close() //nolint:errcheck

	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		// unfortunately we can't use Watch or CachedInformer here as system:node role is only allowed verb 'Get'
		endpoints, err := client.CoreV1().Endpoints(corev1.NamespaceDefault).Get(ctx, "kubernetes", v1.GetOptions{})
		if err != nil {
			return fmt.Errorf("error getting endpoints: %w", err)
		}

		addrs := []netaddr.IP{}

		for _, endpoint := range endpoints.Subsets {
			for _, addr := range endpoint.Addresses {
				ip, err := netaddr.ParseIP(addr.IP)
				if err == nil {
					addrs = append(addrs, ip)
				}
			}
		}

		sort.Slice(addrs, func(i, j int) bool { return addrs[i].Compare(addrs[j]) < 0 })

		if err := r.Modify(ctx,
			k8s.NewEndpoint(k8s.ControlPlaneNamespaceName, k8s.ControlPlaneAPIServerEndpointsID),
			func(r resource.Resource) error {
				if !reflect.DeepEqual(r.(*k8s.Endpoint).TypedSpec().Addresses, addrs) {
					logger.Debug("updated controlplane endpoints", zap.Any("endpoints", addrs))
				}

				r.(*k8s.Endpoint).TypedSpec().Addresses = addrs

				return nil
			},
		); err != nil {
			return fmt.Errorf("error updating endpoints: %w", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
