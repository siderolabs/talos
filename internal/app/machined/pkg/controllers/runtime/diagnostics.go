// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/diagnostics"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// DiagnosticsController analyzes state of Talos Linux system and provides warnings on common problems.
type DiagnosticsController struct{}

// Name implements controller.Controller interface.
func (ctrl *DiagnosticsController) Name() string {
	return "runtime.DiagnosticsController"
}

// Inputs implements controller.Controller interface.
func (ctrl *DiagnosticsController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: k8s.NamespaceName,
			Type:      k8s.NodenameType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *DiagnosticsController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.DiagnosticType,
			Kind: controller.OutputExclusive,
		},
	}
}

const (
	diagnosticsCheckTimeout   = time.Minute
	diagnostricsCheckInterval = time.Minute
)

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *DiagnosticsController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// firstDiscovery is used to track when a warning was first discovered.
	firstDiscovered := map[string]time.Time{}

	ticker := time.NewTicker(diagnostricsCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-ticker.C:
		}

		r.StartTrackingOutputs()

		for _, checkDescription := range diagnostics.Checks() {
			if err := func() error {
				checkCtx, checkCtxCancel := context.WithTimeout(ctx, diagnosticsCheckTimeout)
				defer checkCtxCancel()

				warning, err := checkDescription.Check(checkCtx, r, logger)
				if err != nil {
					logger.Debug("diagnostic check failed", zap.String("check", checkDescription.ID), zap.Error(err))

					return nil
				}

				if warning == nil {
					delete(firstDiscovered, checkDescription.ID)

					return nil
				}

				firstDiscoveredTime, ok := firstDiscovered[checkDescription.ID]
				if !ok {
					firstDiscoveredTime = time.Now()
					firstDiscovered[checkDescription.ID] = firstDiscoveredTime
				}

				if time.Since(firstDiscoveredTime) < checkDescription.Hysteresis {
					// don't publish it yet
					return nil
				}

				return safe.WriterModify(ctx, r, runtime.NewDiagnstic(runtime.NamespaceName, checkDescription.ID), func(res *runtime.Diagnostic) error {
					*res.TypedSpec() = *warning

					return nil
				})
			}(); err != nil {
				return err
			}
		}

		if err := safe.CleanupOutputs[*runtime.Diagnostic](ctx, r); err != nil {
			return err
		}
	}
}
