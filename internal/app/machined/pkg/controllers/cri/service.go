// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"context"
	"fmt"
	"maps"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

const criServiceID = "cri"

// ServiceController owns CRI service startup and configuration-driven restarts.
type ServiceController struct {
	V1Alpha1Services ServiceManager

	versions map[string]string
}

// Name implements controller.Controller interface.
func (ctrl *ServiceController) Name() string {
	return "cri.ServiceController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ServiceController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: files.NamespaceName,
			Type:      files.EtcFileStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ServiceController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *ServiceController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.reconcile(ctx, r); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *ServiceController) reconcile(ctx context.Context, r controller.Runtime) error {
	versions, ready, err := readVersions(ctx, r)
	if err != nil || !ready {
		return err
	}

	running, err := ctrl.isRunning()
	if err != nil {
		return err
	}

	if ctrl.versions == nil {
		return ctrl.initialize(running, versions)
	}

	if maps.Equal(ctrl.versions, versions) {
		return nil
	}

	if running {
		if err = ctrl.restart(ctx); err != nil {
			return err
		}
	} else if err = ctrl.V1Alpha1Services.Start(criServiceID); err != nil {
		return fmt.Errorf("failed to restart CRI service: %w", err)
	}

	ctrl.versions = versions

	return nil
}

func (ctrl *ServiceController) initialize(running bool, versions map[string]string) error {
	if err := ctrl.startIfStopped(running); err != nil {
		return err
	}

	ctrl.versions = versions

	return nil
}

func readVersions(ctx context.Context, r controller.Reader) (map[string]string, bool, error) {
	versions := make(map[string]string, 2)

	for _, id := range []string{constants.CRIConfig, constants.CRIBaseRuntimeSpec} {
		status, err := safe.ReaderGetByID[*files.EtcFileStatus](ctx, r, id)
		if err != nil {
			if state.IsNotFoundError(err) {
				return nil, false, nil
			}

			return nil, false, fmt.Errorf("failed to get etc file status %q: %w", id, err)
		}

		versions[id] = status.TypedSpec().SpecVersion
	}

	return versions, true, nil
}

func (ctrl *ServiceController) isRunning() (bool, error) {
	if ctrl.V1Alpha1Services == nil {
		return false, fmt.Errorf("CRI service manager is not configured")
	}

	_, running, err := ctrl.V1Alpha1Services.IsRunning(criServiceID)
	if err == nil {
		return running, nil
	}

	ctrl.V1Alpha1Services.Load(&services.CRI{})

	return false, nil
}

func (ctrl *ServiceController) startIfStopped(running bool) error {
	if running {
		return nil
	}

	if err := ctrl.V1Alpha1Services.Start(criServiceID); err != nil {
		return fmt.Errorf("failed to start CRI service: %w", err)
	}

	return nil
}

func (ctrl *ServiceController) restart(ctx context.Context) (err error) {
	if err = ctrl.V1Alpha1Services.Stop(ctx, criServiceID); err != nil {
		return fmt.Errorf("failed to stop CRI service: %w", err)
	}

	if err = ctrl.V1Alpha1Services.Start(criServiceID); err != nil {
		return fmt.Errorf("failed to restart CRI service: %w", err)
	}

	return nil
}
