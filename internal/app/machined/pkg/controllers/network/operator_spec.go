// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/operator"
	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// OperatorSpecController applies network.OperatorSpec to the actual interfaces.
type OperatorSpecController struct {
	V1alpha1Platform v1alpha1runtime.Platform
	State            state.State

	// Factory can be overridden for unit-testing.
	Factory OperatorFactory

	operators map[string]*operatorRunState
}

// Name implements controller.Controller interface.
func (ctrl *OperatorSpecController) Name() string {
	return "network.OperatorSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *OperatorSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.OperatorSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.LinkStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *OperatorSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.AddressSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.LinkSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.RouteSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.HostnameSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.ResolverSpecType,
			Kind: controller.OutputShared,
		},
		{
			Type: network.TimeServerSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// operatorRunState describes a state of running operator.
type operatorRunState struct {
	Operator operator.Operator
	Spec     network.OperatorSpecSpec

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (state *operatorRunState) Start(ctx context.Context, notifyCh chan<- struct{}, logger *zap.Logger, id string) {
	state.wg.Add(1)

	ctx, state.cancel = context.WithCancel(ctx)

	go func() {
		defer state.wg.Done()

		state.runWithRestarts(ctx, notifyCh, logger, id)
	}()
}

func (state *operatorRunState) runWithRestarts(ctx context.Context, notifyCh chan<- struct{}, logger *zap.Logger, id string) {
	backoff := backoff.NewExponentialBackOff()

	// disable number of retries limit
	backoff.MaxElapsedTime = 0

	for ctx.Err() == nil {
		if err := state.runWithPanicHandler(ctx, notifyCh, logger, id); err == nil {
			// operator finished without an error
			return
		}

		interval := backoff.NextBackOff()

		logger.Debug("restarting operator", zap.Duration("interval", interval), zap.String("operator", id))

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func (state *operatorRunState) runWithPanicHandler(ctx context.Context, notifyCh chan<- struct{}, logger *zap.Logger, id string) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)

			logger.Error("operator panicked", zap.Stack("stack"), zap.Error(err), zap.String("operator", id))
		}
	}()

	state.Operator.Run(ctx, notifyCh)

	return nil
}

func (state *operatorRunState) Stop() {
	state.cancel()

	state.wg.Wait()
}

// Run implements controller.Controller interface.
func (ctrl *OperatorSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	notifyCh := make(chan struct{})

	ctrl.operators = make(map[string]*operatorRunState)

	defer func() {
		for _, operator := range ctrl.operators {
			operator.Stop()
		}
	}()

	if ctrl.Factory == nil {
		ctrl.Factory = ctrl.newOperator
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			if err := ctrl.reconcileOperators(ctx, r, logger, notifyCh); err != nil {
				return err
			}
		case <-notifyCh:
			if err := ctrl.reconcileOperatorOutputs(ctx, r); err != nil {
				return err
			}
		}

		r.ResetRestartBackoff()
	}
}

//nolint:gocyclo
func (ctrl *OperatorSpecController) reconcileOperators(ctx context.Context, r controller.Runtime, logger *zap.Logger, notifyCh chan<- struct{}) error {
	// build link up statuses
	linkStatuses := make(map[string]bool)

	linkStatusList, err := safe.ReaderListAll[*network.LinkStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing link statuses: %w", err)
	}

	for linkStatus := range linkStatusList.All() {
		linkStatuses[linkStatus.Metadata().ID()] = linkStatus.TypedSpec().OperationalState == nethelpers.OperStateUnknown || linkStatus.TypedSpec().OperationalState == nethelpers.OperStateUp
	}

	// list operator specs
	operatorSpecs, err := safe.ReaderListAll[*network.OperatorSpec](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing operator specs: %w", err)
	}

	// figure out which operators should run
	shouldRun := make(map[string]*network.OperatorSpecSpec)

	for operatorSpec := range operatorSpecs.All() {
		up, exists := linkStatuses[operatorSpec.TypedSpec().LinkName]

		// link doesn't exist, skip operator
		if !exists {
			continue
		}

		// link is down and operator requires link to be up, skip it
		if operatorSpec.TypedSpec().RequireUp && !up {
			continue
		}

		shouldRun[operatorSpec.Metadata().ID()] = operatorSpec.TypedSpec()
	}

	// stop running operators which shouldn't run
	for id := range ctrl.operators {
		if _, exists := shouldRun[id]; !exists {
			logger.Debug("stopping operator", zap.String("operator", id))

			// stop operator
			ctrl.operators[id].Stop()
			delete(ctrl.operators, id)
		} else if !ctrl.operators[id].Spec.Equal(*shouldRun[id]) {
			logger.Debug("replacing operator", zap.String("operator", id))

			// stop operator
			ctrl.operators[id].Stop()
			delete(ctrl.operators, id)
		}
	}

	// start operators which aren't running
	for id := range shouldRun {
		if _, exists := ctrl.operators[id]; !exists {
			ctrl.operators[id] = &operatorRunState{
				Operator: ctrl.Factory(logger, shouldRun[id]),
				Spec:     *shouldRun[id],
			}

			logger.Debug("starting operator", zap.String("operator", id))
			ctrl.operators[id].Start(ctx, notifyCh, logger, id)
		}
	}

	// now reconcile outputs as the operators might have changed
	return ctrl.reconcileOperatorOutputs(ctx, r)
}

//nolint:gocyclo,cyclop
func (ctrl *OperatorSpecController) reconcileOperatorOutputs(ctx context.Context, r controller.Runtime) error {
	r.StartTrackingOutputs()

	for _, op := range ctrl.operators {
		for _, addressSpec := range op.Operator.AddressSpecs() {
			if err := safe.WriterModify(
				ctx, r,
				network.NewAddressSpec(
					network.ConfigNamespaceName,
					fmt.Sprintf("%s/%s", op.Operator.Prefix(), network.AddressID(addressSpec.LinkName, addressSpec.Address)),
				),
				func(r *network.AddressSpec) error {
					*r.TypedSpec() = addressSpec

					return nil
				},
			); err != nil {
				return fmt.Errorf("error applying spec: %w", err)
			}
		}

		for _, routeSpec := range op.Operator.RouteSpecs() {
			if err := safe.WriterModify(
				ctx, r,
				network.NewRouteSpec(
					network.ConfigNamespaceName,
					fmt.Sprintf("%s/%s",
						op.Operator.Prefix(),
						network.RouteID(routeSpec.Table, routeSpec.Family, routeSpec.Destination, routeSpec.Gateway, routeSpec.Priority, routeSpec.OutLinkName),
					),
				),
				func(r *network.RouteSpec) error {
					*r.TypedSpec() = routeSpec

					return nil
				},
			); err != nil {
				return fmt.Errorf("error applying spec: %w", err)
			}
		}

		for _, linkSpec := range op.Operator.LinkSpecs() {
			if err := safe.WriterModify(
				ctx, r,
				network.NewLinkSpec(
					network.ConfigNamespaceName,
					fmt.Sprintf("%s/%s", op.Operator.Prefix(), network.LinkID(linkSpec.Name)),
				),
				func(r *network.LinkSpec) error {
					*r.TypedSpec() = linkSpec

					return nil
				},
			); err != nil {
				return fmt.Errorf("error applying spec: %w", err)
			}
		}

		for _, hostnameSpec := range op.Operator.HostnameSpecs() {
			if err := safe.WriterModify(
				ctx, r,
				network.NewHostnameSpec(
					network.ConfigNamespaceName,
					fmt.Sprintf("%s/%s", op.Operator.Prefix(), network.HostnameID),
				),
				func(r *network.HostnameSpec) error {
					*r.TypedSpec() = hostnameSpec

					return nil
				},
			); err != nil {
				return fmt.Errorf("error applying spec: %w", err)
			}
		}

		for _, resolverSpec := range op.Operator.ResolverSpecs() {
			if err := safe.WriterModify(
				ctx, r,
				network.NewResolverSpec(
					network.ConfigNamespaceName,
					fmt.Sprintf("%s/%s", op.Operator.Prefix(), network.ResolverID),
				),
				func(r *network.ResolverSpec) error {
					*r.TypedSpec() = resolverSpec

					return nil
				},
			); err != nil {
				return fmt.Errorf("error applying spec: %w", err)
			}
		}

		for _, timeserverSpec := range op.Operator.TimeServerSpecs() {
			if err := safe.WriterModify(
				ctx, r,
				network.NewTimeServerSpec(
					network.ConfigNamespaceName,
					fmt.Sprintf("%s/%s", op.Operator.Prefix(), network.TimeServerID),
				),
				func(r *network.TimeServerSpec) error {
					*r.TypedSpec() = timeserverSpec

					return nil
				},
			); err != nil {
				return fmt.Errorf("error applying spec: %w", err)
			}
		}
	}

	// clean up not touched specs
	if err := r.CleanupOutputs(ctx,
		xslices.Map([]resource.Type{
			network.AddressSpecType,
			network.LinkSpecType,
			network.RouteSpecType,
			network.HostnameSpecType,
			network.ResolverSpecType,
			network.TimeServerSpecType,
		}, func(t resource.Type) resource.Kind {
			return resource.NewMetadata(network.ConfigNamespaceName, t, "", resource.VersionUndefined)
		})...,
	); err != nil {
		return fmt.Errorf("error during outputs cleanup: %w", err)
	}

	return nil
}

// OperatorFactory creates operator based on the spec.
type OperatorFactory func(*zap.Logger, *network.OperatorSpecSpec) operator.Operator

func (ctrl *OperatorSpecController) newOperator(logger *zap.Logger, spec *network.OperatorSpecSpec) operator.Operator {
	switch spec.Operator {
	case network.OperatorDHCP4:
		logger = logger.With(zap.String("operator", "dhcp4"))

		return operator.NewDHCP4(logger, spec.LinkName, spec.DHCP4, ctrl.V1alpha1Platform, ctrl.State)
	case network.OperatorDHCP6:
		logger = logger.With(zap.String("operator", "dhcp6"))

		return operator.NewDHCP6(logger, spec.LinkName, spec.DHCP6, ctrl.State)
	case network.OperatorVIP:
		logger = logger.With(zap.String("operator", "vip"))

		return operator.NewVIP(logger, spec.LinkName, spec.VIP, ctrl.State)
	default:
		panic(fmt.Sprintf("unexpected operator %s", spec.Operator))
	}
}
