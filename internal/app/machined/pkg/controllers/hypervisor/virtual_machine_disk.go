// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/disks"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/internal/cleanup"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
	"github.com/siderolabs/talos/pkg/machinery/storagehelpers"
)

const diskControllerName = "hypervisor.VirtualMachineDiskController"

// verifyDigest keeps the domain controller's under-hold verification on the same hashing path
// as disk source observation, without changing its callers or when they verify.
func verifyDigest(reader io.Reader, expected string) error {
	return disks.VerifyDigest(reader, expected)
}

// blankDiskFormats are the formats a volume can be made in: the enum's members less its zero one.
//
// Not VirtualMachineDiskFormatStrings(), and not VirtualMachineDiskFormatString(): enumer puts the
// zero member in its name map, so both accept "unknown", which names no format and which libvirt
// would be handed verbatim.
var blankDiskFormats = []string{
	hypervisorhelpers.VirtualMachineDiskFormatRaw.String(),
	hypervisorhelpers.VirtualMachineDiskFormatQCOW2.String(),
}

// errHoldFailed marks a failure to hold a backing resource (a content library or a storage pool);
// reconcileDisk treats it as a hard error.
var errHoldFailed = errors.New("failed to hold backing resource")

// diskNotReady pairs a not-ready status with the error explaining why.
func diskNotReady(err error) (hypervisor.VirtualMachineDiskStatusSpec, error) {
	return hypervisor.VirtualMachineDiskStatusSpec{
		Phase: hypervisor.VirtualMachineDiskPhaseNotReady,
	}, err
}

// VirtualMachineDiskController resolves each disk of a virtual machine to a host source.
type VirtualMachineDiskController struct{}

// diskReconciliation records the dependencies wanted or held during one event. It is rebuilt
// every pass: Ready sources still need observation, and obsolete held statuses are considered
// separately during teardown before any backing-resource holds are released.
type diskReconciliation struct {
	libraries   map[string]*hypervisor.ContentLibraryStatus
	pools       map[string]*storage.StoragePoolStatus
	wanted      map[resource.ID]struct{}
	held        map[string]struct{}
	heldPools   map[string]struct{}
	volumes     map[resource.ID]struct{}
	wantedFiles map[string]struct{}
}

// Name implements controller.Controller interface.
func (ctrl *VirtualMachineDiskController) Name() string {
	return diskControllerName
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
			Type:      storage.StoragePoolVolumeStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.StoragePoolVolumeSpecType,
			Kind:      controller.InputDestroyReady,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.StoragePoolStatusType,
			// Strong: a materialized disk file lives under a pool's target directory; the pool
			// has to outlive every guest reading or writing one.
			Kind: controller.InputStrong,
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
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-runtime.EventCh():
		}

		if err := ctrl.reconcile(ctx, runtime, logger); err != nil {
			return err
		}

		runtime.ResetRestartBackoff()
	}
}

func (ctrl *VirtualMachineDiskController) reconcile(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger) error {
	specs, err := safe.ReaderListAll[*hypervisor.VirtualMachineSpec](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list virtual machine specs: %w", err)
	}

	libraryStatuses, err := safe.ReaderListAll[*hypervisor.ContentLibraryStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list content library statuses: %w", err)
	}

	libraries := make(map[string]*hypervisor.ContentLibraryStatus, libraryStatuses.Len())

	for library := range libraryStatuses.All() {
		libraries[library.Metadata().ID()] = library
	}

	poolStatuses, err := safe.ReaderListAll[*storage.StoragePoolStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list storage pool statuses: %w", err)
	}

	pools := make(map[string]*storage.StoragePoolStatus, poolStatuses.Len())

	for pool := range poolStatuses.All() {
		pools[pool.Metadata().ID()] = pool
	}

	pass := &diskReconciliation{
		libraries:   libraries,
		pools:       pools,
		wanted:      make(map[resource.ID]struct{}, specs.Len()),
		held:        map[string]struct{}{},
		heldPools:   map[string]struct{}{},
		volumes:     map[resource.ID]struct{}{},
		wantedFiles: map[string]struct{}{},
	}

	var errs []error

	for vm := range specs.All() {
		if vm.Metadata().Phase() != resource.PhaseRunning {
			continue
		}

		name := vm.Metadata().ID()

		for _, disk := range vm.TypedSpec().Disks {
			id := hypervisor.VirtualMachineDiskStatusID(name, disk)
			pass.wanted[id] = struct{}{}

			if err := ctrl.reconcileDisk(ctx, r, logger, name, id, disk, pass); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(append(errs, ctrl.teardown(ctx, r, logger, pass))...)
}

// teardown releases obsolete disk statuses, withdraws volume requests, sweeps unwanted materialized
// files, then releases the pools and libraries nothing resolves against. Pools are released after the
// sweep so cleanup still runs with them mounted.
func (ctrl *VirtualMachineDiskController) teardown(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, pass *diskReconciliation,
) error {
	if err := cleanup.Outputs[*hypervisor.VirtualMachineDiskStatus](ctx, r, "virtual machine disk status", pass.wanted); err != nil {
		return err
	}

	if err := ctrl.releaseVolumeSpecs(ctx, r, pass); err != nil {
		return err
	}

	if err := ctrl.cleanupMaterializedFiles(logger, pass); err != nil {
		return err
	}

	if err := ctrl.releasePools(ctx, r, logger, pass); err != nil {
		return err
	}

	return ctrl.releaseLibraries(ctx, r, logger, pass)
}

// releaseVolumeSpecs withdraws requests no longer present in desired VM intent.
// Attached consumers hold the corresponding statuses until confirmed detach.
func (ctrl *VirtualMachineDiskController) releaseVolumeSpecs(
	ctx context.Context, r controller.ReaderWriter, pass *diskReconciliation,
) error {
	return cleanup.Outputs[*storage.StoragePoolVolumeSpec](ctx, r, "storage pool volume spec", pass.volumes)
}

// reconcileDisk publishes the status of one disk of one virtual machine.
func (ctrl *VirtualMachineDiskController) reconcileDisk(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	name string,
	id resource.ID,
	disk hypervisor.VirtualMachineDiskSpec,
	pass *diskReconciliation,
) error {
	// A status of this exact disk may still be tearing down, held by a domain reading from it:
	// nothing can be written to it, and downstream reads it as absent.
	switch existing, err := safe.ReaderGetByID[*hypervisor.VirtualMachineDiskStatus](ctx, r, id); {
	case err != nil && !state.IsNotFoundError(err):
		return fmt.Errorf("failed to get virtual machine disk status %q: %w", id, err)
	case err == nil && existing.Metadata().Phase() != resource.PhaseRunning:
		return nil
	}

	// The library is held before the status resolved against it is published: a ready disk is one
	// something may start using at any moment.
	resolved, resolveErr := ctrl.progressDisk(ctx, r, logger, name, disk, pass)
	if errors.Is(resolveErr, errHoldFailed) {
		return fmt.Errorf("failed to resolve virtual machine disk %q: %w", id, resolveErr)
	}

	if err := safe.WriterModify(ctx, r,
		hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, id),
		func(res *hypervisor.VirtualMachineDiskStatus) error {
			stampDiskStatus(res.TypedSpec(), name, disk, resolved, resolveErr)

			return nil
		},
	); err != nil {
		return fmt.Errorf("failed to write virtual machine disk status %q: %w", id, err)
	}

	return nil
}

// stampDiskStatus fills in what a disk status says about itself, resolved or not.
//
// What the disk was for is stamped outside the resolution, so a status which did not resolve still
// names its image and its volume. The volume matters doubly: it is what tells a pool its volume is
// still in use, and a status which did not resolve has no source path to go on.
func stampDiskStatus(
	status *hypervisor.VirtualMachineDiskStatusSpec,
	name string,
	disk hypervisor.VirtualMachineDiskSpec,
	resolved hypervisor.VirtualMachineDiskStatusSpec,
	resolveErr error,
) {
	*status = resolved
	status.VirtualMachine = name
	status.Name = disk.Name

	if image := disk.Provision.FromImage; image != nil {
		status.Image = *image
	}

	if disk.Provision.Blank {
		status.Blank = true
		status.Pool = disk.Pool

		if volumeName, err := blankVolumeName(name, disk); err == nil {
			status.Volume = volumeName
		}
	}

	// A materialized disk names its pool too: that is what keeps the pool held while a domain holds
	// this status.
	if disk.Provision.FromImage != nil && disk.Type == hypervisorhelpers.VirtualMachineDiskTypeDisk.String() {
		status.Pool = disk.Pool
	}

	if resolveErr != nil {
		status.Error = resolveErr.Error()
	}
}

// holdBackingResource holds a content library or storage pool for as long as a disk resolves against
// it, so it outlives the guest. One already tearing down is refused, as holding it would block that
// teardown forever. kind and logField only name the resource in messages.
func holdBackingResource(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, md *resource.Metadata, kind, logField string,
) error {
	if md.Phase() != resource.PhaseRunning {
		return fmt.Errorf("%s %q is going away", kind, md.ID())
	}

	if md.Finalizers().Has(diskControllerName) {
		return nil
	}

	if err := r.AddFinalizer(ctx, md, diskControllerName); err != nil {
		return fmt.Errorf("%w: %s %q: %w", errHoldFailed, kind, md.ID(), err)
	}

	logger.Info("holding "+kind+" for a virtual machine disk", zap.String(logField, md.ID()))

	return nil
}

// releaseBackingResources drops the hold on every resource not named by inUse that still carries this
// controller's finalizer.
func releaseBackingResources[T resource.Resource](
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger,
	resources map[string]T, inUse map[string]struct{}, kind, logField string,
) error {
	for id, res := range resources {
		if _, used := inUse[id]; used || !res.Metadata().Finalizers().Has(diskControllerName) {
			continue
		}

		if err := r.RemoveFinalizer(ctx, res.Metadata(), diskControllerName); err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to release %s %q: %w", kind, id, err)
		}

		logger.Info("released "+kind+" held for a virtual machine disk", zap.String(logField, id))
	}

	return nil
}

// statusesInUse names the resources that must stay held: those in held, plus every one named (via
// key) by a disk status something else still holds. key returns "" for a status that names none.
func statusesInUse(
	ctx context.Context, reader controller.Reader, held map[string]struct{},
	key func(*hypervisor.VirtualMachineDiskStatusSpec) string,
) (map[string]struct{}, error) {
	diskStatuses, err := safe.ReaderListAll[*hypervisor.VirtualMachineDiskStatus](ctx, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to list virtual machine disk statuses: %w", err)
	}

	inUse := maps.Clone(held)

	for diskStatus := range diskStatuses.All() {
		if diskStatus.Metadata().Finalizers().Empty() {
			continue
		}

		if id := key(diskStatus.TypedSpec()); id != "" {
			inUse[id] = struct{}{}
		}
	}

	return inUse, nil
}

// releaseLibraries gives back the hold on every library nothing resolves against any more.
func (ctrl *VirtualMachineDiskController) releaseLibraries(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, pass *diskReconciliation,
) error {
	inUse, err := statusesInUse(ctx, r, pass.held,
		func(spec *hypervisor.VirtualMachineDiskStatusSpec) string { return spec.Image.Library })
	if err != nil {
		return err
	}

	return releaseBackingResources(ctx, r, logger, pass.libraries, inUse, "content library", "content_library")
}

// releasePools gives back the hold on every pool no materialized disk resolves against any more; a
// blank disk's pool is held by the storage controller, not here.
func (ctrl *VirtualMachineDiskController) releasePools(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, pass *diskReconciliation,
) error {
	inUse, err := statusesInUse(ctx, r, pass.heldPools, func(spec *hypervisor.VirtualMachineDiskStatusSpec) string {
		if spec.Blank {
			return ""
		}

		return spec.Pool
	})
	if err != nil {
		return err
	}

	return releaseBackingResources(ctx, r, logger, pass.pools, inUse, "storage pool", "storage_pool")
}

// progressDisk validates the disk, publishes its backing-volume request or acquires its library
// hold, then observes the host source. It deliberately runs for Ready disks too; readiness is
// not a cache of a file's digest.
func (ctrl *VirtualMachineDiskController) progressDisk(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	name string,
	disk hypervisor.VirtualMachineDiskSpec,
	pass *diskReconciliation,
) (hypervisor.VirtualMachineDiskStatusSpec, error) {
	if err := checkVirtualMachineDiskSupported(disk); err != nil {
		return diskNotReady(err)
	}

	if disk.Provision.Blank {
		request, err := ctrl.requestBlankVolume(ctx, r, name, disk, pass)
		if err != nil {
			return diskNotReady(err)
		}

		return disks.ObserveVolume(ctx, r, request)
	}

	image := disk.Provision.FromImage

	library, found := pass.libraries[image.Library]
	if !found {
		return diskNotReady(fmt.Errorf("content library %q is not configured", image.Library))
	}

	if err := holdBackingResource(ctx, r, logger, library.Metadata(), "content library", "content_library"); err != nil {
		return diskNotReady(err)
	}

	pass.held[image.Library] = struct{}{}

	if disk.Type == hypervisorhelpers.VirtualMachineDiskTypeDisk.String() {
		return ctrl.progressMaterializedDisk(ctx, r, logger, name, disk, library, pass)
	}

	return disks.ObserveImage(disk, *library.TypedSpec())
}

// progressMaterializedDisk resolves a disk-type disk backed by a content library image. The host
// file lives under the pool's target directory as <vm>/<disk>.qcow2 and is materialized once per
// (vm, disk): linked mode writes a thin qcow2 overlay backed by the library file; copy mode writes
// an independent qcow2 from it. Idempotent at the file level: pre-existing files are left alone.
func (ctrl *VirtualMachineDiskController) progressMaterializedDisk(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	vmName string,
	disk hypervisor.VirtualMachineDiskSpec,
	library *hypervisor.ContentLibraryStatus,
	pass *diskReconciliation,
) (hypervisor.VirtualMachineDiskStatusSpec, error) {
	// vmName is a directory under the pool; reject one that could escape it (disk.Name is checked in
	// checkMaterializedDiskSupported).
	if err := hypervisorhelpers.ValidateName(vmName); err != nil {
		return diskNotReady(fmt.Errorf("%w: virtual machine %w", errDiskUnsupported, err))
	}

	// Resolve and mark the target wanted before touching the source, so a transient library or digest
	// failure below cannot make the cleanup sweep delete an already-materialized file.
	target, err := materializedDiskPath(vmName, disk, pass)
	if err != nil {
		return diskNotReady(err)
	}

	// Hold the pool: InputStrong only permits managing its finalizers, it does not stop the pool being
	// torn down out from under a file a guest is using.
	if err := holdBackingResource(ctx, r, logger, pass.pools[disk.Pool].Metadata(), "storage pool", "storage_pool"); err != nil {
		return diskNotReady(err)
	}

	pass.heldPools[disk.Pool] = struct{}{}

	pass.wantedFiles[target] = struct{}{}

	switch _, statErr := os.Stat(target); {
	case statErr == nil:
		// Already materialized: reconcile only its size, don't re-read or re-hash the source.
		size, err := reconcileDiskSize(ctx, logger, target, disk.Size)
		if err != nil {
			return diskNotReady(err)
		}

		return materializedDiskReady(target, size), nil
	case !errors.Is(statErr, os.ErrNotExist):
		return diskNotReady(fmt.Errorf("stat %q: %w", target, statErr))
	}

	size, err := materializeFromLibrary(ctx, logger, disk, *library.TypedSpec(), target)
	if err != nil {
		return diskNotReady(err)
	}

	return materializedDiskReady(target, size), nil
}

// materializedDiskPath resolves a materialized disk's host path, creating and marking its per-VM
// directory under the (configured, ready) pool.
func materializedDiskPath(vmName string, disk hypervisor.VirtualMachineDiskSpec, pass *diskReconciliation) (string, error) {
	pool, found := pass.pools[disk.Pool]
	if !found {
		return "", fmt.Errorf("storage pool %q is not configured", disk.Pool)
	}

	if pool.TypedSpec().Phase != storage.StoragePoolPhaseReady {
		return "", fmt.Errorf("storage pool %q is not ready: %s", disk.Pool, pool.TypedSpec().Error)
	}

	dir := filepath.Join(pool.TypedSpec().TargetPath, vmName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create pool directory %q: %w", dir, err)
	}

	if err := markVMDir(dir); err != nil {
		return "", err
	}

	return filepath.Join(dir, disk.Name+"."+qcow2Format), nil
}

// materializeFromLibrary creates target from the library image once the source is verified ready,
// intact, and not smaller than requested. It returns the new disk's virtual size: the request, or the
// image's own size when the request is zero.
func materializeFromLibrary(
	ctx context.Context,
	logger *zap.Logger,
	disk hypervisor.VirtualMachineDiskSpec,
	librarySpec hypervisor.ContentLibraryStatusSpec,
	target string,
) (uint64, error) {
	image := disk.Provision.FromImage

	if librarySpec.Phase != hypervisor.ContentLibraryPhaseReady {
		return 0, fmt.Errorf("content library %q is not ready: %s", image.Library, librarySpec.Error)
	}

	if err := disks.CheckLibraryFile(librarySpec.Path, image.File, image.Digest); err != nil {
		return 0, fmt.Errorf("content library %q: file %q: %w", image.Library, image.File, err)
	}

	source := filepath.Join(librarySpec.Path, image.File)

	srcInfo, err := qemuImgInspect(ctx, source)
	if err != nil {
		return 0, fmt.Errorf("inspect image %q: %w", image.File, err)
	}

	// qemu-img cannot build a disk smaller than its source; catch it here with a clear message.
	if disk.Size > 0 && disk.Size < srcInfo.VirtualSize {
		return 0, fmt.Errorf(
			"requested disk size %d is smaller than image %q virtual size %d",
			disk.Size, image.File, srcInfo.VirtualSize)
	}

	if err := materializeDisk(ctx, logger, image.Mode, source, target, disk.Size); err != nil {
		return 0, err
	}

	if disk.Size == 0 {
		return srcInfo.VirtualSize, nil
	}

	return disk.Size, nil
}

// materializedDiskReady is the status of a disk whose host file is present and sized.
func materializedDiskReady(target string, size uint64) hypervisor.VirtualMachineDiskStatusSpec {
	return hypervisor.VirtualMachineDiskStatusSpec{
		SourcePath: target,
		Format:     qcow2Format,
		ReadOnly:   false,
		Phase:      hypervisor.VirtualMachineDiskPhaseReady,
		Size:       size,
	}
}

// reconcileDiskSize reports a materialized disk's actual virtual size, growing the qcow2 in place for
// a larger request and refusing (with a log) a smaller one. requested == 0 inherits the image size.
func reconcileDiskSize(ctx context.Context, logger *zap.Logger, target string, requested uint64) (uint64, error) {
	info, err := qemuImgInspect(ctx, target)
	if err != nil {
		return 0, fmt.Errorf("inspect %q: %w", target, err)
	}

	switch {
	case requested == 0 || requested == info.VirtualSize:
		return info.VirtualSize, nil
	case requested > info.VirtualSize:
		if err := qemuImgResize(ctx, target, requested); err != nil {
			return 0, err
		}

		logger.Info("grew materialized virtual machine disk",
			zap.String("target", target),
			zap.Uint64("from", info.VirtualSize), zap.Uint64("to", requested))

		return requested, nil
	default:
		logger.Warn("refusing to shrink materialized virtual machine disk in place",
			zap.String("target", target),
			zap.Uint64("actual", info.VirtualSize), zap.Uint64("requested", requested))

		return info.VirtualSize, nil
	}
}

// materializeDisk creates target from source in the requested mode (default copy), sizing it to size.
// It writes a sibling temp file renamed into place only once complete, so an interrupted qemu-img run
// never leaves a half-written file the existence check would mistake for a finished disk.
func materializeDisk(ctx context.Context, logger *zap.Logger, mode, source, target string, size uint64) error {
	tmp := target + ".tmp"

	// Drop a temp left by an earlier interrupted attempt before writing a fresh one.
	if err := os.Remove(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale temporary disk %q: %w", tmp, err)
	}

	switch mode {
	case hypervisorhelpers.VirtualMachineDiskImageModeLinked.String():
		if err := qemuImgCreateLinked(ctx, source, tmp, size); err != nil {
			return err
		}
	default:
		if err := qemuImgConvert(ctx, source, tmp); err != nil {
			return err
		}

		if size > 0 {
			if err := qemuImgResize(ctx, tmp, size); err != nil {
				return err
			}
		}
	}

	if err := os.Rename(tmp, target); err != nil {
		return fmt.Errorf("finalize materialized disk %q: %w", target, err)
	}

	logger.Info("materialized virtual machine disk",
		zap.String("mode", mode), zap.String("source", source),
		zap.String("target", target), zap.Uint64("size", size))

	return nil
}

// materializedDiskMarker tags a VM directory this controller created, so the cleanup sweep only ever
// removes its own directories, never a foreign one sharing the pool mount (lost+found, operator-made).
const materializedDiskMarker = ".talos-vm-disk"

// markVMDir records this controller's ownership of dir, creating the marker if absent.
func markVMDir(dir string) error {
	f, err := os.OpenFile(filepath.Join(dir, materializedDiskMarker), os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("mark vm directory %q: %w", dir, err)
	}

	return f.Close()
}

// isMarkedVMDir reports whether dir carries this controller's ownership marker.
func isMarkedVMDir(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, materializedDiskMarker))

	return err == nil
}

// cleanupMaterializedFiles deletes files under every pool's target directory which no wanted disk
// resolves to. The sweep runs on every reconcile, so an orphan survives at most one pass. Only
// VM-scoped subdirectories (one per virtual machine) are touched; sibling files written by the
// storage controller are left alone.
func (ctrl *VirtualMachineDiskController) cleanupMaterializedFiles(
	logger *zap.Logger, pass *diskReconciliation,
) error {
	wantedDirs := map[string]struct{}{}

	for target := range pass.wantedFiles {
		wantedDirs[filepath.Dir(target)] = struct{}{}
	}

	var errs []error

	for id, pool := range pass.pools {
		if pool.TypedSpec().Phase != storage.StoragePoolPhaseReady {
			continue
		}

		errs = append(errs, ctrl.cleanupPoolMaterializedFiles(logger, id, pool.TypedSpec().TargetPath, wantedDirs, pass)...)
	}

	return errors.Join(errs...)
}

// cleanupPoolMaterializedFiles sweeps a single ready pool's target directory, dropping VM
// subdirectories and files that no wanted disk resolves to.
func (ctrl *VirtualMachineDiskController) cleanupPoolMaterializedFiles(
	logger *zap.Logger, id, targetPath string, wantedDirs map[string]struct{}, pass *diskReconciliation,
) []error {
	entries, err := os.ReadDir(targetPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return []error{fmt.Errorf("read pool %q: %w", id, err)}
	}

	var errs []error

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		vmDir := filepath.Join(targetPath, entry.Name())

		// Only sweep directories this controller marked; leave foreign ones (lost+found, operator) alone.
		if !isMarkedVMDir(vmDir) {
			continue
		}

		if _, wanted := wantedDirs[vmDir]; !wanted && !dirHasWantedFiles(vmDir, pass.wantedFiles) {
			// VM has no materialized disks any more; try to drop the whole subdir.
			if err := removeEmptyVMDir(logger, vmDir); err != nil {
				errs = append(errs, err)
			}

			continue
		}

		if err := cleanupMaterializedVMDirectory(logger, id, vmDir, pass.wantedFiles); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

// dirHasWantedFiles reports whether any wanted file lives in dir.
func dirHasWantedFiles(dir string, wanted map[string]struct{}) bool {
	for path := range wanted {
		if filepath.Dir(path) == dir {
			return true
		}
	}

	return false
}

// removeEmptyVMDir drops a VM subdir if it holds no wanted files. Files sitting there are removed
// first so the directory can be emptied before the Remove call.
func removeEmptyVMDir(logger *zap.Logger, vmDir string) error {
	files, err := os.ReadDir(vmDir)
	if err != nil {
		return fmt.Errorf("read vm directory %q: %w", vmDir, err)
	}

	for _, file := range files {
		path := filepath.Join(vmDir, file.Name())

		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove materialized disk %q: %w", path, err)
		}

		logger.Info("removed orphan virtual machine disk file", zap.String("path", path))
	}

	if err := os.Remove(vmDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove empty vm directory %q: %w", vmDir, err)
	}

	return nil
}

func cleanupMaterializedVMDirectory(
	logger *zap.Logger, poolID, vmDir string, wantedFiles map[string]struct{},
) error {
	files, err := os.ReadDir(vmDir)
	if err != nil {
		return fmt.Errorf("read pool %q vm directory %q: %w", poolID, vmDir, err)
	}

	for _, file := range files {
		// Keep the marker as long as the directory lives; removeEmptyVMDir drops it with the directory.
		if file.Name() == materializedDiskMarker {
			continue
		}

		path := filepath.Join(vmDir, file.Name())

		if _, keep := wantedFiles[path]; keep {
			continue
		}

		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove materialized disk %q: %w", path, err)
		}

		logger.Info("removed orphan virtual machine disk file", zap.String("path", path))
	}

	return nil
}

// checkVirtualMachineDiskSupported reports whether a disk is one this slice provisions at all, as
// opposed to one that is merely not resolved yet. Kept apart so the status and the render grade it alike.
//
// Machine configuration validation already rejects most of this. It is checked again because
// VirtualMachineSpec is a shared output: another producer may author one, and nothing binds it to
// the rules a machine configuration document is held to.
func checkVirtualMachineDiskSupported(disk hypervisor.VirtualMachineDiskSpec) error {
	if disk.Provision.Blank && disk.Provision.FromImage != nil {
		return fmt.Errorf("%w: provision: blank and fromImage are mutually exclusive", errDiskUnsupported)
	}

	switch disk.Type {
	case hypervisorhelpers.VirtualMachineDiskTypeCDROM.String():
		switch {
		case disk.Provision.Blank:
			return fmt.Errorf("%w: a cdrom has no contents of its own", errDiskUnsupported)
		case disk.Provision.FromImage == nil:
			return fmt.Errorf("%w: a cdrom requires provision.fromImage", errDiskUnsupported)
		}

		return nil
	case hypervisorhelpers.VirtualMachineDiskTypeDisk.String():
		switch {
		case disk.Provision.Blank:
			return checkBlankDiskSupported(disk)
		case disk.Provision.FromImage != nil:
			return checkMaterializedDiskSupported(disk)
		default:
			return fmt.Errorf("%w: a disk requires provision.blank or provision.fromImage", errDiskUnsupported)
		}
	default:
		return fmt.Errorf("%w: unsupported type %q", errDiskUnsupported, disk.Type)
	}
}

// checkMaterializedDiskSupported reports whether a disk-type disk backed by a content library
// image can be materialized. Format is pinned to qcow2 because both linked and copy modes write
// a qcow2 file; the backing format is detected at create time.
func checkMaterializedDiskSupported(disk hypervisor.VirtualMachineDiskSpec) error {
	if disk.Format != qcow2Format {
		return fmt.Errorf("%w: a materialized disk must be qcow2, got %q", errDiskUnsupported, disk.Format)
	}

	// disk.Name is a filename component under the pool directory; reject one that could escape it.
	if err := hypervisorhelpers.ValidateName(disk.Name); err != nil {
		return fmt.Errorf("%w: disk %w", errDiskUnsupported, err)
	}

	if err := storagehelpers.ValidateStoragePoolName(disk.Pool); err != nil {
		return fmt.Errorf("%w: %w", errDiskUnsupported, err)
	}

	mode := disk.Provision.FromImage.Mode
	switch mode {
	case "",
		hypervisorhelpers.VirtualMachineDiskImageModeCopy.String(),
		hypervisorhelpers.VirtualMachineDiskImageModeLinked.String():
	default:
		return fmt.Errorf("%w: unsupported image mode %q", errDiskUnsupported, mode)
	}

	return nil
}

// checkBlankDiskSupported reports whether a blank disk describes a volume that can be made.
func checkBlankDiskSupported(disk hypervisor.VirtualMachineDiskSpec) error {
	if disk.Size == 0 {
		return fmt.Errorf("%w: a blank disk requires a size", errDiskUnsupported)
	}

	if !slices.Contains(blankDiskFormats, disk.Format) {
		return fmt.Errorf("%w: unsupported format %q, expected one of %v",
			errDiskUnsupported, disk.Format, blankDiskFormats)
	}

	if err := storagehelpers.ValidateStoragePoolName(disk.Pool); err != nil {
		return fmt.Errorf("%w: %w", errDiskUnsupported, err)
	}

	return nil
}

// blankVolumeName preserves filenames for configuration-valid VM and disk names.
// Resource IDs have no such alphabet or length bound. Those use a separate namespace
// (a leading underscore cannot occur in a legacy name) and the full SHA-256 of the
// ID. The disk name remains validated and separated unambiguously. Digest collisions
// are cryptographically improbable, not mathematically impossible; no ID is truncated.
func blankVolumeName(virtualMachine string, disk hypervisor.VirtualMachineDiskSpec) (string, error) {
	if err := hypervisorhelpers.ValidateName(disk.Name); err != nil {
		return "", fmt.Errorf("%w: disk %w", errDiskUnsupported, err)
	}

	if hypervisorhelpers.ValidateName(virtualMachine) == nil {
		return virtualMachine + "__" + disk.Name + "." + disk.Format, nil
	}

	return fmt.Sprintf("_vm-%x__%s.%s", sha256.Sum256([]byte(virtualMachine)), disk.Name, disk.Format), nil
}

// requestBlankVolume publishes the desired backing resource before any source observation.
// Storage alone provisions the volume; marking it wanted before writing also preserves it when
// the write fails. Nothing here takes ownership of storage's finalizers or the volume file.
func (ctrl *VirtualMachineDiskController) requestBlankVolume(
	ctx context.Context,
	r controller.ReaderWriter,
	name string,
	disk hypervisor.VirtualMachineDiskSpec,
	pass *diskReconciliation,
) (*storage.StoragePoolVolumeSpec, error) {
	volumeName, err := blankVolumeName(name, disk)
	if err != nil {
		return nil, err
	}

	request := disks.BlankVolumeSpec(disk, volumeName)
	id := request.Metadata().ID()
	pass.volumes[id] = struct{}{}

	if err = safe.WriterModify(ctx, r,
		request,
		func(spec *storage.StoragePoolVolumeSpec) error {
			*spec.TypedSpec() = *request.TypedSpec()

			return nil
		},
	); err != nil {
		return nil, fmt.Errorf("failed to ask for storage pool volume %q: %w", id, err)
	}

	return request, nil
}
