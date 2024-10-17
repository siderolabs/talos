// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/probe"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// ProbeController runs network probes configured with ProbeSpecs and outputs ProbeStatuses.
type ProbeController struct {
	runners map[string]*probe.Runner
}

// Name implements controller.Controller interface.
func (ctrl *ProbeController) Name() string {
	return "network.ProbeController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ProbeController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.ProbeSpecType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ProbeController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.ProbeStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *ProbeController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	notifyCh := make(chan probe.Notification)

	ctrl.runners = make(map[string]*probe.Runner)

	defer func() {
		for _, runner := range ctrl.runners {
			runner.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			if err := ctrl.reconcileRunners(ctx, r, logger, notifyCh); err != nil {
				return err
			}
		case ev := <-notifyCh:
			if err := ctrl.reconcileOutputs(ctx, r, ev); err != nil {
				return err
			}
		}

		r.ResetRestartBackoff()
	}
}

//nolint:gocyclo
func (ctrl *ProbeController) reconcileRunners(ctx context.Context, r controller.Runtime, logger *zap.Logger, notifyCh chan<- probe.Notification) error {
	specList, err := safe.ReaderListAll[*network.ProbeSpec](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing probe specs: %w", err)
	}

	// figure out which operators should run
	shouldRun := make(map[string]network.ProbeSpecSpec)

	for probeSpec := range specList.All() {
		shouldRun[probeSpec.Metadata().ID()] = *probeSpec.TypedSpec()
	}

	// stop running probes which shouldn't run
	for id := range ctrl.runners {
		if _, exists := shouldRun[id]; !exists {
			logger.Debug("stopping probe", zap.String("probe", id))

			ctrl.runners[id].Stop()
			delete(ctrl.runners, id)
		} else if !shouldRun[id].Equal(ctrl.runners[id].Spec) {
			logger.Debug("replacing probe", zap.String("probe", id))

			ctrl.runners[id].Stop()
			delete(ctrl.runners, id)
		}
	}

	// start probes which aren't running
	for id := range shouldRun {
		if _, exists := ctrl.runners[id]; !exists {
			ctrl.runners[id] = &probe.Runner{
				ID:   id,
				Spec: shouldRun[id],
			}

			logger.Debug("starting probe", zap.String("probe", id))
			ctrl.runners[id].Start(ctx, notifyCh, logger)
		}
	}

	// clean up statuses which should no longer exist
	statusList, err := safe.ReaderListAll[*network.ProbeStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing probe statuses: %w", err)
	}

	for res := range statusList.All() {
		if _, exists := shouldRun[res.Metadata().ID()]; exists {
			continue
		}

		if err = r.Destroy(ctx, res.Metadata()); err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error destroying probe status: %w", err)
		}
	}

	return nil
}

func (ctrl *ProbeController) reconcileOutputs(ctx context.Context, r controller.Runtime, ev probe.Notification) error {
	if _, exists := ctrl.runners[ev.ID]; !exists {
		// probe was already removed, late notification, ignore it
		return nil
	}

	return safe.WriterModify(ctx, r, network.NewProbeStatus(network.NamespaceName, ev.ID),
		func(status *network.ProbeStatus) error {
			*status.TypedSpec() = ev.Status

			return nil
		})
}
