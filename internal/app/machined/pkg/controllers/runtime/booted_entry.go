// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/sdboot"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// BootedEntryController is a controller that updates the booted entry resource.
type BootedEntryController struct {
	V1Alpha1Mode machineruntime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *BootedEntryController) Name() string {
	return "runtime.BootedEntryController"
}

// Inputs implements controller.Controller interface.
func (ctrl *BootedEntryController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      runtimeres.SecurityStateType,
			ID:        optional.Some(runtimeres.SecurityStateID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *BootedEntryController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtimeres.BootedEntryType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *BootedEntryController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	// If we're booted in Container mode, short-circuit the controller.
	if ctrl.V1Alpha1Mode == machineruntime.ModeContainer {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// wait for the SecurityState resource to be created
		st, err := safe.ReaderGetByID[*runtimeres.SecurityState](ctx, r, runtimeres.SecurityStateID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("failed to get machined state: %w", err)
		}

		// If we're not booted with UKI, we don't need to create the BootedEntry resource.
		// This is because the BootedEntry resource is only relevant when UKI is used with systemd-boot.
		if !st.TypedSpec().BootedWithUKI {
			return nil
		}

		// Read `LoaderEntryOneShot`, `LoaderEntryRebootReason`, `LoaderEntrySelected` and `LoaderEntryBooted` resources
		// to determine the booted entry.
		loaderEntryOneShot, err := sdboot.ReadVariable(sdboot.LoaderEntryOneShotName)
		if err != nil {
			return fmt.Errorf("failed to read LoaderEntryOneShot variable: %w", err)
		}

		loaderEntryRebootReason, err := sdboot.ReadVariable(sdboot.LoaderEntryRebootReasonName)
		if err != nil {
			return fmt.Errorf("failed to read LoaderEntryRebootReason variable: %w", err)
		}

		loaderEntrySelected, err := sdboot.ReadVariable(sdboot.LoaderEntrySelectedName)
		if err != nil {
			return fmt.Errorf("failed to read LoaderEntrySelected variable: %w", err)
		}

		loaderEntryDefault, err := sdboot.ReadVariable(sdboot.LoaderEntryDefaultName)
		if err != nil {
			return fmt.Errorf("failed to read LoaderEntryDefault variable: %w", err)
		}

		var bootedEntry string

		switch {
		// in this case `LoaderEntryOneShot` is set to "kexec reboot" and `LoaderEntryRebootReason` is set to "reboot"
		// by the kernel, the system was installed/upgraded via kexec and `LoaderEntryDefault` is the correct booted entry
		// Ref: https://cateee.net/lkddb/web-lkddb/EFI_BOOTLOADER_CONTROL.html
		case loaderEntryRebootReason == "reboot" && loaderEntryOneShot == "kexec reboot":
			if loaderEntryDefault == "" {
				return fmt.Errorf("LoaderEntryDefault variable is empty, cannot determine booted entry")
			}

			bootedEntry = loaderEntryDefault
		// this case is when we have a `LoaderEntryDefault` set by the installer and during a reboot the user selected
		// a different entry, so we set the `LoaderEntrySelected` as the booted entry
		// we can use this information later to decide which UKI's to clean up
		case loaderEntryOneShot == "" && loaderEntryDefault != "" && loaderEntrySelected != "":
			bootedEntry = loaderEntrySelected
		// this case is when we have a `LoaderEntryDefault` set by the installer and the system was rebooted/upgraded
		// with kexec, so `sd-boot` is not involved and nothing sets the `LoaderEntrySelected`
		case loaderEntryOneShot == "" && loaderEntryDefault != "" && loaderEntrySelected == "":
			bootedEntry = loaderEntryDefault
		// this is the case when we just booted with UKI/kernel+initrd and bootloader is not installed
		// this case is only currently applicable when locally developing Talos
		case loaderEntryOneShot == "" && loaderEntryDefault == "" && loaderEntrySelected != "":
			bootedEntry = loaderEntrySelected
		}

		if err := safe.WriterModify(ctx, r, runtimeres.NewBootedEntrySpec(), func(entry *runtimeres.BootedEntry) error {
			entry.TypedSpec().BootedEntry = bootedEntry

			return nil
		}); err != nil {
			return fmt.Errorf("failed to update BootedEntry resource: %w", err)
		}

		// terminating the controller here, as we need to only populate the BootedEntry resource once
		return nil
	}
}
