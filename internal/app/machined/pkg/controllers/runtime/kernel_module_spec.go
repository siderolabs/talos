// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/pmorjan/kmod"
	"go.uber.org/zap"

	v1alpha1runtime "github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/machinery/resources/runtime"
)

// KernelModuleSpecController watches KernelModuleSpecs, sets/resets kernel params.
type KernelModuleSpecController struct {
	V1Alpha1Mode v1alpha1runtime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *KernelModuleSpecController) Name() string {
	return "runtime.KernelModuleSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KernelModuleSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.KernelModuleSpecType,
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *KernelModuleSpecController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *KernelModuleSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Mode == v1alpha1runtime.ModeContainer {
		// not supported in container mode
		return nil
	}

	manager, err := kmod.New()
	if err != nil {
		return fmt.Errorf("error initializing kmod manager: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
			modules, err := r.List(ctx, resource.NewMetadata(runtime.NamespaceName, runtime.KernelModuleSpecType, "", resource.VersionUndefined))
			if err != nil {
				return err
			}

			// note: this code doesn't support module unloading in any way for now
			for _, res := range modules.Items {
				module := res.(*runtime.KernelModuleSpec).TypedSpec()

				if err = manager.Load(module.Name, "", 0); err != nil {
					return fmt.Errorf("error loading module %q: %w", module.Name, err)
				}
			}
		}
	}
}
