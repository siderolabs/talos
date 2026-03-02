// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:revive
package security

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	configres "github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/security"
)

// ImageVerificationConfigController watches machine config and produces ImageVerificationRule resource.
type ImageVerificationConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *ImageVerificationConfigController) Name() string {
	return "security.ImageVerificationConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ImageVerificationConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: configres.NamespaceName,
			Type:      configres.MachineConfigType,
			ID:        optional.Some(configres.ActiveID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ImageVerificationConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: security.ImageVerificationRuleType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *ImageVerificationConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		r.StartTrackingOutputs()

		machineConfig, err := safe.ReaderGetByID[*configres.MachineConfig](ctx, r, configres.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to get machine config: %w", err)
		}

		if machineConfig != nil {
			if cfg := machineConfig.Config().ImageVerificationConfig(); cfg != nil {
				for idx, rule := range cfg.Rules() {
					if err := safe.WriterModify(ctx, r, security.NewImageVerificationRule(fmt.Sprintf("%04d", idx)),
						func(r *security.ImageVerificationRule) error {
							r.TypedSpec().ImagePattern = rule.ImagePattern()
							r.TypedSpec().Verify = rule.Verify()

							if kv := rule.VerifierKeyless(); kv != nil {
								r.TypedSpec().KeylessVerifier = &security.ImageKeylessVerifierSpec{
									Issuer:       kv.Issuer(),
									Subject:      kv.Subject(),
									SubjectRegex: kv.SubjectRegex(),
									RekorURL:     kv.RekorURL(),
								}
							} else {
								r.TypedSpec().KeylessVerifier = nil
							}

							if cv := rule.VerifierPublicKey(); cv != nil {
								r.TypedSpec().PublicKeyVerifier = &security.ImagePublicKeyVerifierSpec{
									Certificate: cv.Certificate(),
								}
							} else {
								r.TypedSpec().PublicKeyVerifier = nil
							}

							return nil
						},
					); err != nil {
						return fmt.Errorf("failed to create/update image verification rule: %w", err)
					}
				}
			}
		}

		if err := safe.CleanupOutputs[*security.ImageVerificationRule](ctx, r); err != nil {
			return fmt.Errorf("failed to cleanup outputs: %w", err)
		}
	}
}
