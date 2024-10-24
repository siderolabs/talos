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
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-loadbalancer/controlplane"
	"github.com/siderolabs/go-loadbalancer/upstream"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// KubePrismController creates KubePrism load balancer based on KubePrismEndpointsType resource.
type KubePrismController struct {
	balancerHost string
	balancerPort int
	lb           *controlplane.LoadBalancer
	ticker       *time.Ticker
	upstreamCh   chan []string
}

// Name implements controller.Controller interface.
func (ctrl *KubePrismController) Name() string {
	return "k8s.KubePrismController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KubePrismController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.KubePrismConfigType,
			ID:        optional.Some(k8s.KubePrismConfigID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *KubePrismController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.KubePrismStatusesType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *KubePrismController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	logger = logger.Named("kubeprism")

	defer func() {
		if ctrl.lb == nil {
			return
		}

		ctrl.stopKubePrism(logger) //nolint:errcheck
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ctrl.takeTickerC():
			err := ctrl.writeKubePrismStatus(ctx, r)
			if err != nil {
				return err
			}

			continue
		case <-r.EventCh():
		}

		lbCfg, err := safe.ReaderGetByID[*k8s.KubePrismConfig](ctx, r, k8s.KubePrismConfigID)
		if err != nil && !state.IsNotFoundError(err) {
			return err
		}

		switch {
		case ctrl.lb == nil && lbCfg != nil:
			err = ctrl.startKubePrism(lbCfg, logger)
			if err != nil {
				return err
			}
		case ctrl.lb != nil && lbCfg == nil:
			err = ctrl.stopKubePrism(logger)
			if err != nil {
				return err
			}
		case ctrl.lb != nil && lbCfg != nil:
			if lbCfg.TypedSpec().Host != ctrl.balancerHost || lbCfg.TypedSpec().Port != ctrl.balancerPort {
				err = ctrl.stopKubePrism(logger)
				if err != nil {
					return err
				}

				err = ctrl.startKubePrism(lbCfg, logger)
				if err != nil {
					return err
				}
			} else {
				ctrl.upstreamChan() <- makeEndpoints(lbCfg.TypedSpec())
			}
		}

		err = ctrl.writeKubePrismStatus(ctx, r)
		if err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

//nolint:gocyclo
func (ctrl *KubePrismController) writeKubePrismStatus(
	ctx context.Context,
	r controller.Runtime,
) error {
	if ctrl.lb != nil && ctrl.endpoint() != "" {
		healthy, err := ctrl.lb.Healthy()
		if err != nil {
			return fmt.Errorf("failed to check KubePrism health: %w", err)
		}

		got, err := safe.ReaderGetByID[*k8s.KubePrismStatuses](ctx, r, k8s.KubePrismStatusesID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get KubePrism status: %w", err)
		}

		if got != nil && got.TypedSpec().Healthy == healthy {
			return nil
		}

		err = safe.WriterModify(
			ctx,
			r,
			k8s.NewKubePrismStatuses(k8s.NamespaceName, k8s.KubePrismStatusesID),
			func(res *k8s.KubePrismStatuses) error {
				res.TypedSpec().Host = ctrl.endpoint()
				res.TypedSpec().Healthy = healthy

				return nil
			},
		)
		if err != nil {
			return fmt.Errorf("failed to write KubePrism status: %w", err)
		}
	}

	// list keys for cleanup
	list, err := safe.ReaderListAll[*k8s.KubePrismStatuses](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing KubePrism resources: %w", err)
	}

	for res := range list.All() {
		if ctrl.lb == nil || res.Metadata().ID() != k8s.KubePrismStatusesID {
			if err = r.Destroy(ctx, res.Metadata()); err != nil {
				return fmt.Errorf("error cleaning up KubePrism specs: %w", err)
			}
		}
	}

	return nil
}

func (ctrl *KubePrismController) startKubePrism(lbCfg *k8s.KubePrismConfig, logger *zap.Logger) error {
	spec := lbCfg.TypedSpec()
	ctrl.balancerHost = spec.Host
	ctrl.balancerPort = spec.Port

	lb, err := controlplane.NewLoadBalancer(ctrl.balancerHost, ctrl.balancerPort,
		logger.WithOptions(zap.IncreaseLevel(zap.ErrorLevel)), // silence the load balancer logs
		controlplane.WithDialTimeout(constants.KubePrismDialTimeout),
		controlplane.WithKeepAlivePeriod(constants.KubePrismKeepAlivePeriod),
		controlplane.WithTCPUserTimeout(constants.KubePrismTCPUserTimeout),
		controlplane.WithHealthCheckOptions(
			upstream.WithHealthcheckInterval(constants.KubePrismHealthCheckInterval),
			upstream.WithHealthcheckTimeout(constants.KubePrismHealthCheckTimeout),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create KubePrism: %w", err)
	}

	err = lb.Start(ctrl.upstreamChan())
	if err != nil {
		return fmt.Errorf("failed to start KubePrism: %w", err)
	}

	logger.Info("KubePrism is enabled", zap.String("endpoint", ctrl.endpoint()))

	ctrl.upstreamChan() <- makeEndpoints(spec)

	ctrl.lb = lb

	return nil
}

func makeEndpoints(spec *k8s.KubePrismConfigSpec) []string {
	return xslices.Map(spec.Endpoints, func(e k8s.KubePrismEndpoint) string {
		return net.JoinHostPort(e.Host, strconv.FormatUint(uint64(e.Port), 10))
	})
}

func (ctrl *KubePrismController) takeTickerC() <-chan time.Time {
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

func (ctrl *KubePrismController) endpoint() string {
	return net.JoinHostPort(ctrl.balancerHost, strconv.FormatUint(uint64(ctrl.balancerPort), 10))
}

func (ctrl *KubePrismController) upstreamChan() chan []string {
	if ctrl.upstreamCh == nil {
		ctrl.upstreamCh = make(chan []string)
	}

	return ctrl.upstreamCh
}

func (ctrl *KubePrismController) stopKubePrism(logger *zap.Logger) error {
	replaceWithZero(&ctrl.upstreamCh)

	lb := replaceWithZero(&ctrl.lb)

	err := lb.Shutdown()
	if err != nil {
		logger.Error("failed to shutdown KubePrism", zap.Error(err))

		return err
	}

	logger.Info("KubePrism is disabled")

	return nil
}

func replaceWithZero[T any](v *T) T {
	var zero T

	result := *v

	*v = zero

	return result
}
