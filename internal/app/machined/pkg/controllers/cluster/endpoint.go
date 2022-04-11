// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"go.uber.org/zap"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/resources/cluster"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
)

// EndpointController looks up control plane endpoints.
type EndpointController struct{}

// Name implements controller.Controller interface.
func (ctrl *EndpointController) Name() string {
	return "cluster.EndpointController"
}

// Inputs implements controller.Controller interface.
func (ctrl *EndpointController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: cluster.NamespaceName,
			Type:      cluster.MemberType,
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

		memberList, err := r.List(ctx, resource.NewMetadata(cluster.NamespaceName, cluster.MemberType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing members: %w", err)
		}

		var endpoints []netaddr.IP

		for _, res := range memberList.Items {
			member := res.(*cluster.TypedResource[cluster.MemberSpec, cluster.Member]).TypedSpec()

			if !(member.MachineType == machine.TypeControlPlane || member.MachineType == machine.TypeInit) {
				continue
			}

			endpoints = append(endpoints, member.Addresses...)
		}

		sort.Slice(endpoints, func(i, j int) bool { return endpoints[i].Compare(endpoints[j]) < 0 })

		if err := r.Modify(ctx,
			k8s.NewEndpoint(k8s.ControlPlaneNamespaceName, k8s.ControlPlaneDiscoveredEndpointsID),
			func(r resource.Resource) error {
				if !reflect.DeepEqual(r.(*k8s.Endpoint).TypedSpec().Addresses, endpoints) {
					logger.Debug("updated controlplane endpoints", zap.Any("endpoints", endpoints))
				}

				r.(*k8s.Endpoint).TypedSpec().Addresses = endpoints

				return nil
			},
		); err != nil {
			return fmt.Errorf("error updating endpoints: %w", err)
		}
	}
}
