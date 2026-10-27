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
	"path/filepath"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/disks"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/volumerequest"
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

// errHoldFailed marks the controller's own failure to hold a library.
var errHoldFailed = errors.New("failed to hold content library")

// VirtualMachineDiskController resolves each disk of a virtual machine to a host source.
type VirtualMachineDiskController struct{}

// diskReconciliation records the dependencies wanted or held during one event. It is rebuilt
// every pass: Ready sources still need observation, and obsolete held statuses are considered
// separately during teardown before any backing-resource holds are released.
type diskReconciliation struct {
	libraries map[string]*hypervisor.ContentLibraryStatus
	wanted    map[resource.ID]struct{}
	held      map[string]struct{}
	volumes   map[resource.ID]struct{}
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

	pass := &diskReconciliation{
		libraries: libraries,
		wanted:    make(map[resource.ID]struct{}, specs.Len()),
		held:      map[string]struct{}{},
		volumes:   map[resource.ID]struct{}{},
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

// teardown first asks domains to release obsolete disk statuses, then withdraws volume requests,
// then releases libraries. Held statuses continue protecting both kinds of backing resource.
func (ctrl *VirtualMachineDiskController) teardown(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, pass *diskReconciliation,
) error {
	if err := cleanup.Outputs[*hypervisor.VirtualMachineDiskStatus](ctx, r, "virtual machine disk status", pass.wanted); err != nil {
		return err
	}

	if err := ctrl.releaseVolumeSpecs(ctx, r, pass); err != nil {
		return err
	}

	return ctrl.releaseLibraries(ctx, r, logger, pass)
}

// releaseVolumeSpecs withdraws requests no longer present in desired VM intent.
// Attached consumers hold the corresponding statuses until confirmed detach.
func (ctrl *VirtualMachineDiskController) releaseVolumeSpecs(
	ctx context.Context, r controller.ReaderWriter, pass *diskReconciliation,
) error {
	requests, err := safe.ReaderListAll[*storage.StoragePoolVolumeSpec](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list storage pool volume specs: %w", err)
	}

	for request := range requests.All() {
		md := request.Metadata()
		if !volumerequest.Owned(request, ctrl.Name()) {
			continue
		}

		if _, keep := pass.volumes[md.ID()]; keep && md.Phase() == resource.PhaseRunning {
			continue
		}

		if err := teardownPoolVolumeRequest(ctx, r, request); err != nil {
			return err
		}
	}

	return nil
}

func teardownPoolVolumeRequest(ctx context.Context, r controller.ReaderWriter, request *storage.StoragePoolVolumeSpec) error {
	if !volumerequest.Owned(request, diskControllerName) {
		return nil
	}

	ready, err := r.Teardown(ctx, request.Metadata())
	if state.IsNotFoundError(err) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("failed to tear down storage pool volume spec %q: %w", request.Metadata().ID(), err)
	}

	if !ready {
		return nil
	}

	if err := r.Destroy(ctx, request.Metadata()); err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("failed to destroy storage pool volume spec %q: %w", request.Metadata().ID(), err)
	}

	return nil
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

	// Hold the provisioning source only until storage publishes an independent pool copy.
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

	if disk.Provision.Blank || disk.Provision.FromImage != nil {
		status.Pool = disk.Pool
		if disk.Provision.Blank {
			status.Blank = true
		}

		if volumeName, err := diskVolumeName(name, disk); err == nil {
			status.Volume = volumeName
		}
	}

	if resolveErr != nil {
		status.Error = resolveErr.Error()
	}
}

// holdLibrary protects the provisioning source while storage copies it. A library already
// tearing down is refused rather than held, so a new copy cannot prevent its teardown.
func holdLibrary(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, library *hypervisor.ContentLibraryStatus,
) error {
	if library.Metadata().Phase() != resource.PhaseRunning {
		return fmt.Errorf("content library %q is going away", library.Metadata().ID())
	}

	if library.Metadata().Finalizers().Has(diskControllerName) {
		return nil
	}

	if err := r.AddFinalizer(ctx, library.Metadata(), diskControllerName); err != nil {
		return fmt.Errorf("%w %q: %w", errHoldFailed, library.Metadata().ID(), err)
	}

	logger.Info("holding content library for a virtual machine disk", zap.String("content_library", library.Metadata().ID()))

	return nil
}

// releaseLibraries gives back the hold on every library nothing resolves against any more.
func (ctrl *VirtualMachineDiskController) releaseLibraries(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger,
	pass *diskReconciliation,
) error {
	for id, library := range pass.libraries {
		if _, used := pass.held[id]; used || !library.Metadata().Finalizers().Has(diskControllerName) {
			continue
		}

		if err := r.RemoveFinalizer(ctx, library.Metadata(), diskControllerName); err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to release content library %q: %w", id, err)
		}

		logger.Info("released content library held for a virtual machine disk", zap.String("content_library", id))
	}

	return nil
}

// progressDisk validates a disk, publishes its backing-volume intent, then observes storage.
// A Ready pool-owned copy never reopens the library; only an incomplete initial copy needs a
// held source. Blank disks are also owned by storage and remain create-only.
func (ctrl *VirtualMachineDiskController) progressDisk(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	name string,
	disk hypervisor.VirtualMachineDiskSpec,
	pass *diskReconciliation,
) (hypervisor.VirtualMachineDiskStatusSpec, error) {
	if err := checkVirtualMachineDiskSupported(disk); err != nil {
		return hypervisor.VirtualMachineDiskStatusSpec{
			Phase: hypervisor.VirtualMachineDiskPhaseNotReady,
		}, err
	}

	if disk.Provision.Blank {
		request, err := ctrl.requestBlankVolume(ctx, r, name, disk, pass)
		if err != nil {
			return hypervisor.VirtualMachineDiskStatusSpec{
				Phase: hypervisor.VirtualMachineDiskPhaseNotReady,
			}, err
		}

		return disks.ObserveVolume(ctx, r, request)
	}

	request, err := ctrl.requestImageVolume(ctx, r, name, disk, pass)
	if err != nil {
		return hypervisor.VirtualMachineDiskStatusSpec{Phase: hypervisor.VirtualMachineDiskPhaseNotReady}, err
	}

	return ctrl.progressImageSource(ctx, r, logger, disk, request, pass)
}

func (ctrl *VirtualMachineDiskController) requestImageVolume(
	ctx context.Context, r controller.ReaderWriter, name string,
	disk hypervisor.VirtualMachineDiskSpec, pass *diskReconciliation,
) (*storage.StoragePoolVolumeSpec, error) {
	volumeName, err := diskVolumeName(name, disk)
	if err != nil {
		return nil, err
	}

	request := storage.NewStoragePoolVolumeSpec(storage.NamespaceName, storage.StoragePoolVolumeID(disk.Pool, volumeName))
	*request.TypedSpec() = storage.StoragePoolVolumeSpecSpec{
		Pool:   disk.Pool,
		Name:   volumeName,
		Format: disk.Format,
		Copy:   &storage.StoragePoolVolumeCopySpec{},
	}
	pass.volumes[request.Metadata().ID()] = struct{}{}

	// Publish intent even after a restart with no ephemeral volume spec and no library. The
	// storage slice can rediscover an owned copy from the pool without reopening its source.
	if err = safe.WriterModify(ctx, r, request, func(current *storage.StoragePoolVolumeSpec) error {
		if err := volumerequest.EnsureAdoption(current, ctrl.Name(), volumerequest.Destination{
			Pool:   disk.Pool,
			Name:   volumeName,
			Format: disk.Format,
		}); err != nil {
			return fmt.Errorf("volume %q belongs to another request", request.Metadata().ID())
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return request, nil
}

func holdImageCopySource(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger,
	library *hypervisor.ContentLibraryStatus, request *storage.StoragePoolVolumeSpec,
) error {
	if request.TypedSpec().Copy.Source != "" && library.Metadata().Finalizers().Has(diskControllerName) {
		return nil
	}

	return holdLibrary(ctx, r, logger, library)
}

func (ctrl *VirtualMachineDiskController) progressImageSource(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, disk hypervisor.VirtualMachineDiskSpec,
	request *storage.StoragePoolVolumeSpec, pass *diskReconciliation,
) (hypervisor.VirtualMachineDiskStatusSpec, error) {
	image := disk.Provision.FromImage
	resolved, observeErr := disks.ObserveVolume(ctx, r, request)

	resolved.ReadOnly = disk.Type == hypervisorhelpers.VirtualMachineDiskTypeCDROM.String()
	if observeErr == nil {
		return resolved, nil
	}

	current, err := safe.ReaderGetByID[*storage.StoragePoolVolumeSpec](ctx, r, request.Metadata().ID())
	if err != nil {
		return resolved, err
	}

	library, found := pass.libraries[image.Library]
	if !found {
		if resolved.Phase == hypervisor.VirtualMachineDiskPhaseObservationUnavailable {
			return resolved, observeErr
		}

		return hypervisor.VirtualMachineDiskStatusSpec{Phase: hypervisor.VirtualMachineDiskPhaseNotReady},
			fmt.Errorf("content library %q is not configured: %w", image.Library, observeErr)
	}

	if err := holdImageCopySource(ctx, r, logger, library, current); err != nil {
		return hypervisor.VirtualMachineDiskStatusSpec{Phase: hypervisor.VirtualMachineDiskPhaseNotReady}, err
	}

	pass.held[image.Library] = struct{}{}

	if current.TypedSpec().Copy.Source != "" || resolved.Phase == hypervisor.VirtualMachineDiskPhaseObservationUnavailable {
		return resolved, observeErr
	}

	source, err := disks.ObserveImage(disk, *library.TypedSpec())
	if err != nil {
		return source, err
	}

	if err := bindImageCopySource(ctx, r, request, disk, source); err != nil {
		return hypervisor.VirtualMachineDiskStatusSpec{Phase: hypervisor.VirtualMachineDiskPhaseNotReady}, err
	}

	return resolved, observeErr
}

func bindImageCopySource(ctx context.Context, r controller.ReaderWriter,
	request *storage.StoragePoolVolumeSpec, disk hypervisor.VirtualMachineDiskSpec, source hypervisor.VirtualMachineDiskStatusSpec,
) error {
	return safe.WriterModify(ctx, r, request, func(current *storage.StoragePoolVolumeSpec) error {
		if err := volumerequest.BindImage(current, diskControllerName, volumerequest.Destination{
			Pool: disk.Pool,
			Name: request.TypedSpec().Name,
		}, source.SourcePath, "sha256:"+filepath.Base(source.SourcePath)); err != nil {
			return fmt.Errorf("volume %q belongs to another request", request.Metadata().ID())
		}

		return nil
	})
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
		return checkCDROMDiskSupported(disk)
	case hypervisorhelpers.VirtualMachineDiskTypeDisk.String():
		if !disk.Provision.Blank {
			return fmt.Errorf("%w: a disk is only provisioned from provision.blank today", errDiskUnsupported)
		}

		return checkBlankDiskSupported(disk)
	default:
		return fmt.Errorf("%w: unsupported type %q", errDiskUnsupported, disk.Type)
	}
}

func checkCDROMDiskSupported(disk hypervisor.VirtualMachineDiskSpec) error {
	if disk.Provision.FromImage == nil || disk.Provision.Blank {
		return fmt.Errorf("%w: a cdrom requires provision.fromImage", errDiskUnsupported)
	}

	if mode := disk.Provision.FromImage.Mode; mode != "" && mode != hypervisorhelpers.VirtualMachineDiskImageModeCopy.String() {
		return fmt.Errorf("%w: unsupported image mode %q for cdrom", errDiskUnsupported, mode)
	}

	if err := storagehelpers.ValidateStoragePoolName(disk.Pool); err != nil {
		return fmt.Errorf("%w: %w", errDiskUnsupported, err)
	}

	if disk.Format != hypervisorhelpers.VirtualMachineDiskFormatRaw.String() {
		return fmt.Errorf("%w: cdrom copy requires raw format", errDiskUnsupported)
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

// diskVolumeName keeps writable guest disks stable across source edits, but gives read-only media
// a different destination for each explicit image identity. A CDROM must never attach the copy
// from a prior File or Digest merely because its device name did not change.
func diskVolumeName(virtualMachine string, disk hypervisor.VirtualMachineDiskSpec) (string, error) {
	base, err := blankVolumeName(virtualMachine, disk)
	if err != nil || disk.Type != hypervisorhelpers.VirtualMachineDiskTypeCDROM.String() || disk.Provision.FromImage == nil {
		return base, err
	}

	image := disk.Provision.FromImage
	sum := sha256.Sum256([]byte(image.Library + "\x00" + image.File + "\x00" + image.Digest))

	return base[:len(base)-len(disk.Format)-1] + fmt.Sprintf("__%x.%s", sum, disk.Format), nil
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
