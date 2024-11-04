// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"context"
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

//go:embed data/ca-certificates
var defaultCACertificates []byte

// TrustedRootsController manages CA trusted roots based on configuration.
type TrustedRootsController struct{}

// Name implements controller.Controller interface.
func (ctrl *TrustedRootsController) Name() string {
	return "secrets.TrustedRootsController"
}

// Inputs implements controller.Controller interface.
func (ctrl *TrustedRootsController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *TrustedRootsController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: files.EtcFileSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *TrustedRootsController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get machine config: %w", err)
		}

		contents := slices.Clone(defaultCACertificates)

		if cfg != nil {
			contents = slices.Concat(contents, []byte(strings.Join(cfg.Config().TrustedRoots().ExtraTrustedRootCertificates(), "\n\n")))
		}

		if err = safe.WriterModify(ctx, r, files.NewEtcFileSpec(files.NamespaceName, constants.DefaultTrustedRelativeCAFile), func(spec *files.EtcFileSpec) error {
			spec.TypedSpec().Mode = 0o644
			spec.TypedSpec().Contents = contents

			return nil
		}); err != nil {
			return fmt.Errorf("failed to write trusted roots: %w", err)
		}

		r.ResetRestartBackoff()
	}
}
