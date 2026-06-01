// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// APIServiceConfigController provides apid service configuration.
type APIServiceConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *APIServiceConfigController) Name() string {
	return "runtime.APIServiceConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *APIServiceConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MaintenanceServiceRequestType,
			ID:        optional.Some(runtime.MaintenanceServiceRequestID),
			Kind:      controller.InputStrong,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MaintenanceServiceConfigType,
			ID:        optional.Some(runtime.MaintenanceServiceConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: secrets.NamespaceName,
			Type:      secrets.APIType,
			ID:        optional.Some(secrets.APIID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *APIServiceConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.APIServiceConfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *APIServiceConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		request, err := safe.ReaderGetByID[*runtime.MaintenanceServiceRequest](ctx, r, runtime.MaintenanceServiceRequestID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get maintenance service request: %w", err)
		}

		if request != nil && request.Metadata().Phase() == resource.PhaseTearingDown {
			// remove the finalizer
			if err = r.RemoveFinalizer(ctx, request.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("failed to remove finalizer: %w", err)
			}

			request = nil
		}

		// immediately add a finalizer
		if request != nil {
			if err = r.AddFinalizer(ctx, request.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("failed to add finalizer: %w", err)
			}
		}

		cfg, err := safe.ReaderGetByID[*runtime.MaintenanceServiceConfig](ctx, r, runtime.MaintenanceServiceConfigID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get maintenance service config: %w", err)
		}

		cert, err := safe.ReaderGetByID[*secrets.API](ctx, r, secrets.APIID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get API secret: %w", err)
		}

		r.StartTrackingOutputs()

		// decide whether to create maintenance mode API or not
		if request != nil {
			if cfg != nil {
				if err = safe.WriterModify(
					ctx, r,
					runtime.NewAPIServiceConfig(),
					func(r *runtime.APIServiceConfig) error {
						r.TypedSpec().ListenAddress = cfg.TypedSpec().ListenAddress
						r.TypedSpec().NodeRoutingDisabled = true
						r.TypedSpec().ReadonlyRoleMode = true
						r.TypedSpec().SkipVerifyingClientCert = true

						return nil
					},
				); err != nil {
					return fmt.Errorf("failed to create API service config: %w", err)
				}
			}
		} else if cert != nil && !cert.TypedSpec().SkipVerifyingClientCert {
			if err = safe.WriterModify(
				ctx, r,
				runtime.NewAPIServiceConfig(),
				func(r *runtime.APIServiceConfig) error {
					r.TypedSpec().ListenAddress = fmt.Sprintf(":%d", constants.ApidPort)
					r.TypedSpec().NodeRoutingDisabled = false
					r.TypedSpec().ReadonlyRoleMode = false
					r.TypedSpec().SkipVerifyingClientCert = false

					return nil
				},
			); err != nil {
				return fmt.Errorf("failed to create API service config: %w", err)
			}
		}

		if err = safe.CleanupOutputs[*runtime.APIServiceConfig](ctx, r); err != nil {
			return fmt.Errorf("failed to cleanup API service config outputs: %w", err)
		}
	}
}
