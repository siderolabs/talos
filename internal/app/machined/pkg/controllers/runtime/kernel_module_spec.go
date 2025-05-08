// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/pmorjan/kmod"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
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
func (ctrl *KernelModuleSpecController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	if ctrl.V1Alpha1Mode == v1alpha1runtime.ModeContainer {
		// not supported in container mode
		return nil
	}

	manager, err := kmod.New(kmod.SetInitFunc(finitMod))
	if err != nil {
		return fmt.Errorf("error initializing kmod manager: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		modules, err := safe.ReaderListAll[*runtime.KernelModuleSpec](ctx, r)
		if err != nil {
			return err
		}

		var multiErr error

		// note: this code doesn't support module unloading in any way for now
		for module := range modules.All() {
			moduleSpec := module.TypedSpec()
			parameters := strings.Join(moduleSpec.Parameters, " ")

			if err = manager.Load(moduleSpec.Name, parameters, 0); err != nil {
				multiErr = errors.Join(multiErr, fmt.Errorf("error loading module %q: %w", moduleSpec.Name, err))
			}
		}

		if multiErr != nil {
			return multiErr
		}

		r.ResetRestartBackoff()
	}
}

// finitMod loads a kernel module, with finit_module(2). Compressed modules are supported via
// in-kernel decompression by passing the `MODULE_INIT_COMPRESSED_FILE` flag.
func finitMod(filename string, params string, flags int) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	//nolint:errcheck
	defer f.Close()

	return unix.FinitModule(int(f.Fd()), params, flags|unix.MODULE_INIT_COMPRESSED_FILE)
}
