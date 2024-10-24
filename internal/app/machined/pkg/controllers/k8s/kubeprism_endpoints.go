// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/transform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// KubePrismEndpointsController creates a list of API server endpoints.
type KubePrismEndpointsController = transform.Controller[*config.MachineConfig, *k8s.KubePrismEndpoints]

// NewKubePrismEndpointsController instanciates the controller.
//
//nolint:gocyclo
func NewKubePrismEndpointsController() *KubePrismEndpointsController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *k8s.KubePrismEndpoints]{
			Name: "k8s.KubePrismEndpointsController",
			MapMetadataOptionalFunc: func(cfg *config.MachineConfig) optional.Optional[*k8s.KubePrismEndpoints] {
				if cfg.Metadata().ID() != config.V1Alpha1ID {
					return optional.None[*k8s.KubePrismEndpoints]()
				}

				if cfg.Config().Cluster() == nil || cfg.Config().Machine() == nil {
					return optional.None[*k8s.KubePrismEndpoints]()
				}

				return optional.Some(k8s.NewKubePrismEndpoints(k8s.NamespaceName, k8s.KubePrismEndpointsID))
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, machineConfig *config.MachineConfig, res *k8s.KubePrismEndpoints) error {
				members, err := safe.ReaderListAll[*cluster.Member](ctx, r)
				if err != nil {
					return fmt.Errorf("error listing affiliates: %w", err)
				}

				var endpoints []k8s.KubePrismEndpoint

				ce := machineConfig.Config().Cluster().Endpoint()
				if ce != nil {
					endpoints = append(endpoints, k8s.KubePrismEndpoint{
						Host: ce.Hostname(),
						Port: toPort(ce.Port()),
					})
				}

				if machineConfig.Config().Machine().Type().IsControlPlane() {
					endpoints = append(endpoints, k8s.KubePrismEndpoint{
						Host: "localhost",
						Port: uint32(machineConfig.Config().Cluster().LocalAPIServerPort()),
					})
				}

				for member := range members.All() {
					memberSpec := member.TypedSpec()

					if len(memberSpec.Addresses) > 0 && memberSpec.ControlPlane != nil {
						for _, addr := range memberSpec.Addresses {
							endpoints = append(endpoints, k8s.KubePrismEndpoint{
								Host: addr.String(),
								Port: uint32(memberSpec.ControlPlane.APIServerPort),
							})
						}
					}
				}

				res.TypedSpec().Endpoints = endpoints

				return nil
			},
		},
		transform.WithExtraInputs(
			safe.Input[*cluster.Member](controller.InputWeak),
		),
	)
}
