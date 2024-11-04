// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/extensions"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// ExtensionStatusController loads extensions.yaml and updates ExtensionStatus resources.
type ExtensionStatusController struct{}

// Name implements controller.Controller interface.
func (ctrl *ExtensionStatusController) Name() string {
	return "runtime.ExtensionStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ExtensionStatusController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *ExtensionStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.ExtensionStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *ExtensionStatusController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	// controller runs once, as extensions are static
	select {
	case <-ctx.Done():
		return nil
	case <-r.EventCh():
	}

	var cfg extensions.Config

	if err := cfg.Read(constants.ExtensionsRuntimeConfigFile); err != nil {
		if errors.Is(err, io.EOF) {
			// no extensions installed
			return nil
		}

		return fmt.Errorf("failed loading extensions config: %w", err)
	}

	for _, layer := range cfg.Layers {
		id := strings.TrimSuffix(layer.Image, ".sqsh")

		if err := safe.WriterModify(ctx, r, runtime.NewExtensionStatus(runtime.NamespaceName, id), func(res *runtime.ExtensionStatus) error {
			*res.TypedSpec() = *layer

			return nil
		}); err != nil {
			return err
		}
	}

	return nil
}
