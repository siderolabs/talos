// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	blockadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/block"
	secretsadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/secrets"
	"github.com/siderolabs/talos/internal/app/machined/pkg/automaton/blockautomaton"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	"github.com/siderolabs/talos/pkg/xfs"
)

// EncryptionSaltController manages secrets.EncryptionSalt in STATE.
type EncryptionSaltController struct {
	stateMachine blockautomaton.VolumeMounterAutomaton
}

// Name implements controller.Controller interface.
func (ctrl *EncryptionSaltController) Name() string {
	return "secrets.EncryptionSaltController"
}

// Inputs implements controller.Controller interface.
func (ctrl *EncryptionSaltController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountStatusType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountRequestType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *EncryptionSaltController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: secrets.EncryptionSaltType,
			Kind: controller.OutputShared,
		},
		{
			Type: block.VolumeMountRequestType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *EncryptionSaltController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if ctrl.stateMachine == nil {
			ctrl.stateMachine = blockautomaton.NewVolumeMounter(
				ctrl.Name(),
				constants.StatePartitionLabel,
				ctrl.establishEncryptionSalt,
				blockautomaton.WithDetached(true),
			)
		}

		if err := ctrl.stateMachine.Run(ctx, r, logger); err != nil {
			return fmt.Errorf("error running volume mounter machine: %w", err)
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *EncryptionSaltController) establishEncryptionSalt(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, mountStatus *block.VolumeMountStatus) error {
	return blockadapter.VolumeMountStatus(mountStatus).WithRoot(logger, func(root xfs.Root) error {
		var salt secrets.EncryptionSaltSpec

		if err := controllers.LoadOrNewFromFile(root, constants.EncryptionSaltFilename, &salt, func(v *secrets.EncryptionSaltSpec) error {
			return secretsadapter.EncryptionSalt(v).Generate()
		}); err != nil {
			return fmt.Errorf("error caching node identity: %w", err)
		}

		if err := safe.WriterModify(ctx, r, secrets.NewEncryptionSalt(), func(r *secrets.EncryptionSalt) error {
			*r.TypedSpec() = salt

			return nil
		}); err != nil {
			return fmt.Errorf("error modifying resource: %w", err)
		}

		// encryption salt thumbprint for debugging purposes
		saltHMAC := hmac.New(sha256.New, salt.DiskSalt)
		saltHMAC.Write([]byte("encryption salt checksum"))
		saltChecksum := saltHMAC.Sum(nil)

		logger.Info("encryption salt established",
			zap.String("salt_checksum", hex.EncodeToString(saltChecksum)),
		)

		return nil
	})
}
