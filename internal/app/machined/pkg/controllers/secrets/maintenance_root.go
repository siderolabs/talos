// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/crypto/x509"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// MaintenanceRootController manages secrets.Root based on configuration.
type MaintenanceRootController struct{}

// Name implements controller.Controller interface.
func (ctrl *MaintenanceRootController) Name() string {
	return "secrets.MaintenanceRootController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MaintenanceRootController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *MaintenanceRootController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: secrets.MaintenanceRootType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *MaintenanceRootController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	// run this controller only once, as the CA never changes
	select {
	case <-ctx.Done():
		return nil
	case <-r.EventCh():
	}

	return safe.WriterModify(ctx, r, secrets.NewMaintenanceRoot(secrets.MaintenanceRootID), func(root *secrets.MaintenanceRoot) error {
		ca, err := x509.NewSelfSignedCertificateAuthority()
		if err != nil {
			return fmt.Errorf("failed to generate self-signed CA: %w", err)
		}

		root.TypedSpec().CA = x509.NewCertificateAndKeyFromCertificateAuthority(ca)

		return nil
	})
}
