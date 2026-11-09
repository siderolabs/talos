// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/disks"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// VirtualMachineDiskController resolves each disk of a virtual machine to a host source.
type VirtualMachineDiskController struct{}

// Name implements controller.Controller interface.
func (ctrl *VirtualMachineDiskController) Name() string {
	return disks.ControllerName
}

// Inputs implements controller.Controller interface.
func (ctrl *VirtualMachineDiskController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.ContentLibraryStatusType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDiskStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.StoragePoolStatusType,
			Kind:      controller.InputWeak,
		},
		// Copy observations and requests whose consumers have detached.
		{
			Namespace: storage.NamespaceName,
			Type:      storage.StoragePoolVolumeStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.StoragePoolVolumeSpecType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *VirtualMachineDiskController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hypervisor.VirtualMachineDiskStatusType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: storage.StoragePoolVolumeSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *VirtualMachineDiskController) Run(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	reconciler := disks.NewReconciler(&disks.ImageVerifier{})

	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()

	var retry <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-runtime.EventCh():
		case <-retry:
			retry = nil
		}

		if err := reconciler.Reconcile(ctx, runtime, logger); err != nil {
			return err
		}

		retry = scheduleSourceRetry(timer, retry, reconciler.SourceRetryNeeded())

		runtime.ResetRestartBackoff()
	}
}

func scheduleSourceRetry(timer *time.Timer, retry <-chan time.Time, needed bool) <-chan time.Time {
	if needed {
		if retry == nil {
			timer.Reset(time.Second)
		}

		return timer.C
	}

	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}

	return nil
}
