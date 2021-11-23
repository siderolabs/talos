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
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network/operator"
	v1alpha1runtime "github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
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
	}
}

//nolint:gocyclo
func (ctrl *OperatorSpecController) reconcileOperators(ctx context.Context, r controller.Runtime, logger *zap.Logger, notifyCh chan<- struct{}) error {
	// build link up statuses
	linkStatuses := make(map[string]bool)

	list, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.LinkStatusType, "", resource.VersionUndefined))
	if err != nil {
		return fmt.Errorf("error listing source network addresses: %w", err)
	}

	for _, item := range list.Items {
		linkStatus := item.(*network.LinkStatus) //nolint:errcheck,forcetypeassert

		linkStatuses[linkStatus.Metadata().ID()] = linkStatus.TypedSpec().OperationalState == nethelpers.OperStateUnknown || linkStatus.TypedSpec().OperationalState == nethelpers.OperStateUp
	}

	// list operator specs
	list, err = r.List(ctx, resource.NewMetadata(network.NamespaceName, network.OperatorSpecType, "", resource.VersionUndefined))
	if err != nil {
		return fmt.Errorf("error listing source network addresses: %w", err)
	}

	// figure out which operators should run
	shouldRun := make(map[string]*network.OperatorSpecSpec)

	for _, item := range list.Items {
		operatorSpec := item.(*network.OperatorSpec) //nolint:errcheck,forcetypeassert

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
		} else if *shouldRun[id] != ctrl.operators[id].Spec {
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
	// query specs from all operators and update outputs
	touchedIDs := map[string]map[string]struct{}{}

	apply := func(res resource.Resource, fn func(resource.Resource)) error {
		if touchedIDs[res.Metadata().Type()] == nil {
			touchedIDs[res.Metadata().Type()] = map[string]struct{}{}
		}

		touchedIDs[res.Metadata().Type()][res.Metadata().ID()] = struct{}{}

		return r.Modify(ctx, res, func(r resource.Resource) error {
			fn(r)

			return nil
		})
	}

	for _, op := range ctrl.operators {
		for _, addressSpec := range op.Operator.AddressSpecs() {
			addressSpec := addressSpec

			if err := apply(
				network.NewAddressSpec(
					network.ConfigNamespaceName,
					fmt.Sprintf("%s/%s", op.Operator.Prefix(), network.AddressID(addressSpec.LinkName, addressSpec.Address)),
				),
				func(r resource.Resource) {
					*r.(*network.AddressSpec).TypedSpec() = addressSpec
				},
			); err != nil {
				return fmt.Errorf("error applying spec: %w", err)
			}
		}

		for _, routeSpec := range op.Operator.RouteSpecs() {
			routeSpec := routeSpec

			if err := apply(
				network.NewRouteSpec(
					network.ConfigNamespaceName,
					fmt.Sprintf("%s/%s", op.Operator.Prefix(), network.RouteID(routeSpec.Table, routeSpec.Family, routeSpec.Destination, routeSpec.Gateway, routeSpec.Priority)),
				),
				func(r resource.Resource) {
					*r.(*network.RouteSpec).TypedSpec() = routeSpec
				},
			); err != nil {
				return fmt.Errorf("error applying spec: %w", err)
			}
		}

		for _, linkSpec := range op.Operator.LinkSpecs() {
			linkSpec := linkSpec

			if err := apply(
				network.NewLinkSpec(
					network.ConfigNamespaceName,
					fmt.Sprintf("%s/%s", op.Operator.Prefix(), network.LinkID(linkSpec.Name)),
				),
				func(r resource.Resource) {
					*r.(*network.LinkSpec).TypedSpec() = linkSpec
				},
			); err != nil {
				return fmt.Errorf("error applying spec: %w", err)
			}
		}

		for _, hostnameSpec := range op.Operator.HostnameSpecs() {
			hostnameSpec := hostnameSpec

			if err := apply(
				network.NewHostnameSpec(
					network.ConfigNamespaceName,
					fmt.Sprintf("%s/%s", op.Operator.Prefix(), network.HostnameID),
				),
				func(r resource.Resource) {
					*r.(*network.HostnameSpec).TypedSpec() = hostnameSpec
				},
			); err != nil {
				return fmt.Errorf("error applying spec: %w", err)
			}
		}

		for _, resolverSpec := range op.Operator.ResolverSpecs() {
			resolverSpec := resolverSpec

			if err := apply(
				network.NewResolverSpec(
					network.ConfigNamespaceName,
					fmt.Sprintf("%s/%s", op.Operator.Prefix(), network.ResolverID),
				),
				func(r resource.Resource) {
					*r.(*network.ResolverSpec).TypedSpec() = resolverSpec
				},
			); err != nil {
				return fmt.Errorf("error applying spec: %w", err)
			}
		}

		for _, timeserverSpec := range op.Operator.TimeServerSpecs() {
			timeserverSpec := timeserverSpec

			if err := apply(
				network.NewTimeServerSpec(
					network.ConfigNamespaceName,
					fmt.Sprintf("%s/%s", op.Operator.Prefix(), network.TimeServerID),
				),
				func(r resource.Resource) {
					*r.(*network.TimeServerSpec).TypedSpec() = timeserverSpec
				},
			); err != nil {
				return fmt.Errorf("error applying spec: %w", err)
			}
		}
	}

	// clean up not touched specs
	for _, resourceType := range []resource.Type{
		network.AddressSpecType,
		network.LinkSpecType,
		network.RouteSpecType,
		network.HostnameSpecType,
		network.ResolverSpecType,
		network.TimeServerSpecType,
	} {
		list, err := r.List(ctx, resource.NewMetadata(network.ConfigNamespaceName, resourceType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing specs: %w", err)
		}

		for _, item := range list.Items {
			if item.Metadata().Owner() != ctrl.Name() {
				continue
			}

			touched := false

			if touchedIDs[resourceType] != nil {
				if _, exists := touchedIDs[resourceType][item.Metadata().ID()]; exists {
					touched = true
				}
			}

			if !touched {
				if err = r.Destroy(ctx, item.Metadata()); err != nil {
					return fmt.Errorf("error cleaning up untouched spec: %w", err)
				}
			}
		}
	}

	return nil
}

// OperatorFactory creates operator based on the spec.
type OperatorFactory func(*zap.Logger, *network.OperatorSpecSpec) operator.Operator

func (ctrl *OperatorSpecController) newOperator(logger *zap.Logger, spec *network.OperatorSpecSpec) operator.Operator {
	switch spec.Operator {
	case network.OperatorDHCP4:
		logger = logger.With(zap.String("operator", "dhcp4"))

		return operator.NewDHCP4(logger, spec.LinkName, spec.DHCP4.RouteMetric, ctrl.V1alpha1Platform)
	case network.OperatorDHCP6:
		logger = logger.With(zap.String("operator", "dhcp6"))

		return operator.NewDHCP6(logger, spec.LinkName)
	case network.OperatorVIP:
		logger = logger.With(zap.String("operator", "vip"))

		return operator.NewVIP(logger, spec.LinkName, spec.VIP, ctrl.State)
	default:
		panic(fmt.Sprintf("unexpected operator %s", spec.Operator))
	}
}
