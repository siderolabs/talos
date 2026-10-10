// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package disks

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// BlankVolumeSpec constructs the desired backing resource without observing or provisioning it.
// The controller validates the disk and volume name before constructing this request.
func BlankVolumeSpec(disk hypervisor.VirtualMachineDiskSpec, volumeName string) *storage.StoragePoolVolumeSpec {
	request := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, storage.StoragePoolVolumeID(disk.Pool, volumeName))
	*request.TypedSpec() = storage.StoragePoolVolumeSpecSpec{
		Pool:     disk.Pool,
		Name:     volumeName,
		Capacity: disk.Size,
		Format:   disk.Format,
	}

	return request
}

// ObserveVolume reads the storage slice's answer after the controller has published the request.
// Physical provisioning remains with storage; no host file is created here.
func ObserveVolume(
	ctx context.Context,
	reader controller.Reader,
	request *storage.StoragePoolVolumeSpec,
) (hypervisor.VirtualMachineDiskStatusSpec, error) {
	id := request.Metadata().ID()
	pool := request.TypedSpec().Pool
	volumeName := request.TypedSpec().Name
	volumeStatus, err := safe.ReaderGetByID[*storage.StoragePoolVolumeStatus](ctx, reader, id)

	switch {
	case state.IsNotFoundError(err):
		return hypervisor.VirtualMachineDiskStatusSpec{
			Phase: hypervisor.VirtualMachineDiskPhaseNotReady,
		}, fmt.Errorf("waiting for volume %q in storage pool %q", volumeName, pool)
	case err != nil:
		return hypervisor.VirtualMachineDiskStatusSpec{
			Phase: hypervisor.VirtualMachineDiskPhaseNotReady,
		}, fmt.Errorf("failed to get storage pool volume status %q: %w", id, err)
	case volumeStatus.Metadata().Phase() != resource.PhaseRunning:
		return hypervisor.VirtualMachineDiskStatusSpec{
			Phase: hypervisor.VirtualMachineDiskPhaseNotReady,
		}, fmt.Errorf("volume %q is going away", id)
	case volumeStatus.TypedSpec().Phase != storage.StoragePoolVolumePhaseReady:
		phase := hypervisor.VirtualMachineDiskPhaseNotReady
		if volumeStatus.TypedSpec().Phase == storage.StoragePoolVolumePhaseObservationUnavailable {
			phase = hypervisor.VirtualMachineDiskPhaseObservationUnavailable
		}

		volumeErr := fmt.Errorf("volume %q in storage pool %q is not ready: %s", volumeName, pool, volumeStatus.TypedSpec().Error)

		return hypervisor.VirtualMachineDiskStatusSpec{
			Phase:      phase,
			SourcePath: volumeStatus.TypedSpec().Path,
			Format:     volumeStatus.TypedSpec().Format,
			Size:       volumeStatus.TypedSpec().Capacity,
		}, volumeErr
	}

	return hypervisor.VirtualMachineDiskStatusSpec{
		SourcePath: volumeStatus.TypedSpec().Path,
		Format:     volumeStatus.TypedSpec().Format,
		// A blank disk is the guest's to write into; that is the whole point of it.
		ReadOnly: false,
		Phase:    hypervisor.VirtualMachineDiskPhaseReady,
		Size:     volumeStatus.TypedSpec().Capacity,
	}, nil
}
