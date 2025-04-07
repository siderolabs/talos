// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/ecks/uefi/efi/efivario"
	"github.com/foxboron/go-uefi/efi"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/sdboot"
	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// SecurityStateController is a controller that updates the security state of Talos.
type SecurityStateController struct {
	V1Alpha1Mode machineruntime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *SecurityStateController) Name() string {
	return "runtime.SecurityStateController"
}

// Inputs implements controller.Controller interface.
func (ctrl *SecurityStateController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			Kind:      controller.OutputExclusive,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *SecurityStateController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtimeres.SecurityStateType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *SecurityStateController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// wait for the `machined` service to start, as by that time initial mounts will be done
		_, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, r, "machined")
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("failed to get machined state: %w", err)
		}

		var (
			secureBootState          bool
			bootedWithUKI            bool
			pcrSigningKeyFingerprint string
		)

		// in container mode, never populate the fields
		if ctrl.V1Alpha1Mode != machineruntime.ModeContainer {
			if efi.GetSecureBoot() && !efi.GetSetupMode() {
				secureBootState = true
			}

			efiVarCtx := efivario.NewDefaultContext()

			defaultEntry, err := sdboot.ReadVariable(efiVarCtx, sdboot.LoaderEntryDefaultName)
			if err == nil {
				if strings.HasPrefix(defaultEntry, "Talos-") {
					bootedWithUKI = true
				}
			}

			// if defaultEntry is empty in the case when we booted off a disk image when installer never runs, we can rely on the
			// stub image identifier to determine if we booted with UKI
			if defaultEntry == "" {
				stubImageIdentifier, err := sdboot.ReadVariable(efiVarCtx, sdboot.StubImageIdentifierName)
				if err == nil {
					if strings.HasPrefix(filepath.Base(strings.ReplaceAll(stubImageIdentifier, "\\", "/")), "Talos-") {
						bootedWithUKI = true
					}
				}
			}

			if pcrPublicKeyData, err := os.ReadFile(constants.PCRPublicKey); err == nil {
				block, _ := pem.Decode(pcrPublicKeyData)
				if block == nil {
					return errors.New("failed to decode PEM block for PCR public key")
				}

				cert := x509.Certificate{
					Raw: block.Bytes,
				}

				pcrSigningKeyFingerprint = x509CertFingerprint(cert)
			}
		}

		selinuxState, err := getSelinuxState()
		if err != nil {
			return fmt.Errorf("failed to get SELinux state: %w", err)
		}

		if err := safe.WriterModify(ctx, r, runtimeres.NewSecurityStateSpec(runtimeres.NamespaceName), func(state *runtimeres.SecurityState) error {
			state.TypedSpec().SecureBoot = secureBootState
			state.TypedSpec().PCRSigningKeyFingerprint = pcrSigningKeyFingerprint
			state.TypedSpec().SELinuxState = selinuxState
			state.TypedSpec().BootedWithUKI = bootedWithUKI

			return nil
		}); err != nil {
			return err
		}

		// terminating the controller here, as we need to only populate securitystate once
		return nil
	}
}

func x509CertFingerprint(cert x509.Certificate) string {
	hash := sha256.Sum256(cert.Raw)

	var buf bytes.Buffer

	for i, b := range hex.EncodeToString(hash[:]) {
		if i > 0 && i%2 == 0 {
			buf.WriteByte(':')
		}

		buf.WriteString(strings.ToUpper(string(b)))
	}

	return buf.String()
}

func getSelinuxState() (runtimeres.SELinuxState, error) {
	if !selinux.IsEnabled() {
		return runtimeres.SELinuxStateDisabled, nil
	}

	// Read /sys/fs/selinux/enforce to determine if SELinux is in enforcing mode
	// Make sure LSM mode is actually enforcing, in case we later allow setenforce
	// IsEnabled is reliable, since LSM is active whenever SELinuxFS is mounted, which is done accordingly
	data, err := os.ReadFile("/sys/fs/selinux/enforce")
	if err != nil {
		return runtimeres.SELinuxStateDisabled, err
	}

	if strings.TrimSpace(string(data)) == "1" {
		return runtimeres.SELinuxStateEnforcing, nil
	}

	return runtimeres.SELinuxStatePermissive, nil
}
