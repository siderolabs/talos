// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/slices"
	"github.com/siderolabs/go-loadbalancer/controlplane"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// APILoadBalancerController creates load balancer based on APIServerEndpointsType resource.
type APILoadBalancerController struct {
	balancerHost string
	balancerPort int
	lb           *controlplane.LoadBalancer
	ticker       *time.Ticker
	upstreamCh   chan []string
}

// Name implements controller.Controller interface.
func (ctrl *APILoadBalancerController) Name() string {
	return "k8s.APILoadBalancerController"
}

// Inputs implements controller.Controll
// er interface.
func (ctrl *APILoadBalancerController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.LoadBalancerConfigType,
			ID:        pointer.To(k8s.LoadBalancerConfigID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *APILoadBalancerController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.LoadBalancerStatusesType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *APILoadBalancerController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	logger = logger.Named("api-endpoints-balancer")

	defer func() {
		if ctrl.lb == nil {
			return
		}

		ctrl.stopLoadBalancer(logger) //nolint:errcheck
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ctrl.takeTickerC():
			err := ctrl.writeLoadbalancerStatus(ctx, r)
			if err != nil {
				return err
			}

			continue
		case <-r.EventCh():
		}

		lbCfg, err := safe.ReaderGetByID[*k8s.LoadBalancerConfig](ctx, r, k8s.LoadBalancerConfigID)
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		switch {
		case ctrl.lb == nil && lbCfg != nil:
			err = ctrl.startLoadBalancer(lbCfg, logger)
			if err != nil {
				return err
			}
		case ctrl.lb != nil && lbCfg == nil:
			err = ctrl.stopLoadBalancer(logger)
			if err != nil {
				return err
			}
		case ctrl.lb != nil && lbCfg != nil:
			if lbCfg.TypedSpec().Host != ctrl.balancerHost || lbCfg.TypedSpec().Port != ctrl.balancerPort {
				err = ctrl.stopLoadBalancer(logger)
				if err != nil {
					return err
				}

				err = ctrl.startLoadBalancer(lbCfg, logger)
				if err != nil {
					return err
				}
			} else {
				ctrl.upstreamChan() <- makeEndpoints(lbCfg.TypedSpec())
			}
		}

		err = ctrl.writeLoadbalancerStatus(ctx, r)
		if err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

//nolint:gocyclo
func (ctrl *APILoadBalancerController) writeLoadbalancerStatus(
	ctx context.Context,
	r controller.Runtime,
) error {
	if ctrl.lb != nil && ctrl.endpoint() != "" {
		healthy, err := ctrl.lb.Healthy()
		if err != nil {
			return fmt.Errorf("failed to check load balancer health: %w", err)
		}

		got, err := safe.ReaderGetByID[*k8s.LoadBalancerStatuses](ctx, r, k8s.LoadBalancerStatusesID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get load balancer status: %w", err)
		}

		if got != nil && got.TypedSpec().Healthy == healthy {
			return nil
		}

		err = safe.WriterModify(
			ctx,
			r,
			k8s.NewLoadBalancerStatuses(k8s.NamespaceName, k8s.LoadBalancerStatusesID),
			func(res *k8s.LoadBalancerStatuses) error {
				res.TypedSpec().Host = ctrl.endpoint()
				res.TypedSpec().Healthy = healthy

				return nil
			},
		)
		if err != nil {
			return fmt.Errorf("failed to write load balancer status: %w", err)
		}
	}

	// list keys for cleanup
	list, err := safe.ReaderListAll[*k8s.LoadBalancerStatuses](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing resources: %w", err)
	}

	for it := safe.IteratorFromList(list); it.Next(); {
		res := it.Value()

		if ctrl.lb == nil || res.Metadata().ID() != k8s.LoadBalancerStatusesID {
			if err = r.Destroy(ctx, res.Metadata()); err != nil {
				return fmt.Errorf("error cleaning up specs: %w", err)
			}
		}
	}

	return nil
}

func (ctrl *APILoadBalancerController) startLoadBalancer(lbCfg *k8s.LoadBalancerConfig, logger *zap.Logger) error {
	spec := lbCfg.TypedSpec()
	ctrl.balancerHost = spec.Host
	ctrl.balancerPort = spec.Port

	lb, err := controlplane.NewLoadBalancer(ctrl.balancerHost, ctrl.balancerPort,
		logger.WithOptions(zap.IncreaseLevel(zap.ErrorLevel)), // silence the load balancer logs
	)
	if err != nil {
		return fmt.Errorf("failed to create load balancer: %w", err)
	}

	err = lb.Start(ctrl.upstreamChan())
	if err != nil {
		return fmt.Errorf("failed to start load balancer: %w", err)
	}

	logger.Info("api server load balancer is enabled", zap.String("endpoint", ctrl.endpoint()))

	ctrl.upstreamChan() <- makeEndpoints(spec)

	ctrl.lb = lb

	return nil
}

func makeEndpoints(spec *k8s.LoadBalancerConfigSpec) []string {
	return slices.Map(spec.Endpoints, func(e k8s.APIServerEndpoint) string {
		return net.JoinHostPort(e.Host, strconv.FormatUint(uint64(e.Port), 10))
	})
}

func (ctrl *APILoadBalancerController) takeTickerC() <-chan time.Time {
	switch {
	case ctrl.lb == nil && ctrl.ticker == nil:
		return nil
	case ctrl.lb != nil && ctrl.ticker == nil:
		ctrl.ticker = time.NewTicker(5 * time.Second)

		return ctrl.ticker.C
	case ctrl.lb == nil:
		ticker := replaceWithZero(&ctrl.ticker)
		if ticker != nil {
			ticker.Stop()
		}

		return nil
	default:
		return ctrl.ticker.C
	}
}

func (ctrl *APILoadBalancerController) endpoint() string {
	return net.JoinHostPort(ctrl.balancerHost, strconv.FormatUint(uint64(ctrl.balancerPort), 10))
}

func (ctrl *APILoadBalancerController) upstreamChan() chan []string {
	if ctrl.upstreamCh == nil {
		ctrl.upstreamCh = make(chan []string)
	}

	return ctrl.upstreamCh
}

func (ctrl *APILoadBalancerController) stopLoadBalancer(logger *zap.Logger) error {
	replaceWithZero(&ctrl.upstreamCh)

	lb := replaceWithZero(&ctrl.lb)

	err := lb.Shutdown()
	if err != nil {
		logger.Error("failed to shutdown the load balancer", zap.Error(err))

		return err
	}

	logger.Info("api server load balancer is disabled")

	return nil
}

func replaceWithZero[T any](v *T) T {
	var zero T

	result := *v

	*v = zero

	return result
}
