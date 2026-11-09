// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package disks

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// RetainCopySource holds an incomplete copy's source library and returns its ID.
// A qualified completed status avoids binding reads. Observation errors are returned
// alongside the retained ID so the caller can preserve the hold while reporting them.
// The request must carry a copy spec.
func RetainCopySource(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	request *storage.StoragePoolVolumeSpec,
	libraries map[resource.ID]*hypervisor.ContentLibraryStatus,
) (resource.ID, error) {
	if request.TypedSpec().Copy.Source == "" {
		return "", nil
	}

	completed, readErr := completedImageCopySource(ctx, r, request)
	if completed {
		return "", nil
	}

	library := libraries[request.TypedSpec().Copy.SourceLibrary]
	if library == nil {
		return "", errors.Join(readErr, fmt.Errorf("cannot establish source library for pending copy %q", request.Metadata().ID()))
	}

	if holdErr := holdImageCopySource(ctx, r, logger, library, request); holdErr != nil {
		return "", errors.Join(readErr, holdErr)
	}

	return library.Metadata().ID(), readErr
}

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

	if request.TypedSpec().Copy != nil {
		poolStatus, poolErr := safe.ReaderGetByID[*storage.StoragePoolStatus](ctx, reader, pool)
		if poolErr != nil || !CopyPublicationCompleted(request, volumeStatus, poolStatus) {
			return hypervisor.VirtualMachineDiskStatusSpec{
				Phase: hypervisor.VirtualMachineDiskPhaseNotReady,
			}, fmt.Errorf("volume %q copy is not qualified for current intent", id)
		}
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

// CopyPublicationCompleted qualifies durable initialization even when observation
// is temporarily unavailable. It releases source lifetime, never authorizes attachment.
// The caller supplies existing request, status and pool resources.
func CopyPublicationCompleted(request *storage.StoragePoolVolumeSpec, status *storage.StoragePoolVolumeStatus, pool *storage.StoragePoolStatus) bool {
	observed := status.TypedSpec().Copy

	return request.Metadata().Phase() == resource.PhaseRunning && status.Metadata().Phase() == resource.PhaseRunning &&
		storage.StoragePoolVolumeCopyCompletionMatches(request, pool, observed) &&
		copyVolumeIdentityMatches(request.TypedSpec(), status.TypedSpec(), pool.TypedSpec())
}

func copyVolumeIdentityMatches(request *storage.StoragePoolVolumeSpecSpec, status *storage.StoragePoolVolumeStatusSpec, pool *storage.StoragePoolStatusSpec) bool {
	return (status.Phase == storage.StoragePoolVolumePhaseReady || status.Phase == storage.StoragePoolVolumePhaseObservationUnavailable) &&
		status.Pool == request.Pool &&
		status.Name == request.Name &&
		status.Format == request.Format &&
		pool.TargetPath != "" &&
		status.Path == filepath.Join(pool.TargetPath, request.Name)
}
