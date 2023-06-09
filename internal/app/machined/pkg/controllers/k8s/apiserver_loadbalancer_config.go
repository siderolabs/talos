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

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// APILoadBalancerConfigController creates config for load balancer.
type APILoadBalancerConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *APILoadBalancerConfigController) Name() string {
	return "k8s.APILoadBalancerConfigController"
}

// Inputs implements controller.Controll
// er interface.
func (ctrl *APILoadBalancerConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.APIServerEndpointsType,
			ID:        pointer.To(k8s.APIServerEndpointsID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.To(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *APILoadBalancerConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.LoadBalancerConfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *APILoadBalancerConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		if _, ok := channel.RecvWithContext(ctx, r.EventCh()); !ok && ctx.Err() != nil {
			return nil //nolint:nilerr
		}

		endpt, err := safe.ReaderGetByID[*k8s.APIServerEndpoints](ctx, r, k8s.APIServerEndpointsID)
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		mc, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		wroteConfig, err := ctrl.writeConfig(ctx, r, endpt, mc)
		if err != nil {
			return err
		}

		// list keys for cleanup
		lbCfgList, err := safe.ReaderListAll[*k8s.LoadBalancerConfig](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for it := safe.IteratorFromList(lbCfgList); it.Next(); {
			res := it.Value()

			if !wroteConfig || res.Metadata().ID() != k8s.LoadBalancerConfigID {
				if err = r.Destroy(ctx, res.Metadata()); err != nil {
					return fmt.Errorf("error cleaning up load balancer config: %w", err)
				}
			}
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *APILoadBalancerConfigController) writeConfig(ctx context.Context, r controller.Runtime, endpt *k8s.APIServerEndpoints, mc *config.MachineConfig) (bool, error) {
	if endpt == nil || mc == nil {
		return false, nil
	}

	endpoints := endpt.TypedSpec().Endpoints
	if len(endpoints) == 0 {
		return false, nil
	}

	balancerCfg := mc.Config().Machine().Features().APIServerBalancer()
	if !balancerCfg.Enabled() {
		return false, nil
	}

	err := safe.WriterModify(
		ctx,
		r,
		k8s.NewLoadBalancerConfig(k8s.NamespaceName, k8s.LoadBalancerConfigID),
		func(res *k8s.LoadBalancerConfig) error {
			spec := res.TypedSpec()
			spec.Endpoints = endpoints
			spec.Host = "localhost"
			spec.Port = balancerCfg.Port()

			return nil
		},
	)
	if err != nil {
		return false, fmt.Errorf("failed to write load balancer config: %w", err)
	}

	return true, nil
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
