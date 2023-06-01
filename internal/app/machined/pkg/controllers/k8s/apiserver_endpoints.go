// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// APIServerEndpointsController creates a list of API server endpoints.
type APIServerEndpointsController struct{}

// Name implements controller.Controller interface.
func (ctrl *APIServerEndpointsController) Name() string {
	return "cluster.APIServerEndpointsController"
}

// Inputs implements controller.Controller interface.
func (ctrl *APIServerEndpointsController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        pointer.To(config.MachineTypeID),
			Kind:      controller.InputWeak,
		},
		safe.Input[*cluster.Member](controller.InputWeak),
		safe.Input[*config.MachineConfig](controller.InputWeak),
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *APIServerEndpointsController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.APIServerEndpointsType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *APIServerEndpointsController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		if _, ok := channel.RecvWithContext(ctx, r.EventCh()); !ok && ctx.Err() != nil {
			return nil //nolint:nilerr
		}

		machineConfig, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting machine config: %w", err)
			}

			continue
		}

		machineType, err := safe.ReaderGetByID[*config.MachineType](ctx, r, config.MachineTypeID)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting machine type: %w", err)
			}

			continue
		}

		members, err := safe.ReaderListAll[*cluster.Member](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing affiliates: %w", err)
		}

		var endpoints []k8s.APIServerEndpoint

		ce := machineConfig.Config().Cluster().Endpoint()
		if ce != nil {
			endpoints = append(endpoints, k8s.APIServerEndpoint{
				Host: ce.Hostname(),
				Port: toPort(ce.Port()),
			})
		}

		if machineType.MachineType() == machine.TypeControlPlane {
			endpoints = append(endpoints, k8s.APIServerEndpoint{
				Host: "localhost",
				Port: uint32(machineConfig.Config().Cluster().LocalAPIServerPort()),
			})
		}

		for it := safe.IteratorFromList(members); it.Next(); {
			memberSpec := it.Value().TypedSpec()

			if len(memberSpec.Addresses) > 0 && memberSpec.ControlPlane != nil {
				for _, addr := range memberSpec.Addresses {
					endpoints = append(endpoints, k8s.APIServerEndpoint{
						Host: addr.String(),
						Port: uint32(memberSpec.ControlPlane.APIServerPort),
					})
				}
			}
		}

		err = safe.WriterModify[*k8s.APIServerEndpoints](
			ctx,
			r,
			k8s.NewEndpoints(k8s.NamespaceName, k8s.APIServerEndpointsID),
			func(res *k8s.APIServerEndpoints) error {
				res.TypedSpec().Endpoints = endpoints

				return nil
			},
		)
		if err != nil {
			return fmt.Errorf("error updating endpoints: %w", err)
		}

		// list keys for cleanup
		list, err := safe.ReaderListAll[*k8s.APIServerEndpoints](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for it := safe.IteratorFromList(list); it.Next(); {
			res := it.Value()

			if res.Metadata().Owner() != ctrl.Name() {
				continue
			}

			if res.Metadata().ID() != k8s.APIServerEndpointsID {
				if err = r.Destroy(ctx, res.Metadata()); err != nil {
					return fmt.Errorf("error cleaning up specs: %w", err)
				}

				logger.Info("removed endpoints resource", zap.String("id", res.Metadata().ID()))
			}
		}

		r.ResetRestartBackoff()
	}
}

func toPort(port string) uint32 {
	if port == "" {
		return 443
	}

	p, err := strconv.ParseUint(port, 10, 32)
	if err != nil {
		return 443
	}

	return uint32(p)
}
