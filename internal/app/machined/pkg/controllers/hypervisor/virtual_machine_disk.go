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

	if resolveErr != nil {
		status.Error = resolveErr.Error()
	}
}

// holdLibrary keeps a library's mount in place for as long as a disk resolves against it. One which
// is tearing down is refused rather than held, as holding it now would block that teardown forever.
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
	inUse, err := librariesInUse(ctx, r, pass.held)
	if err != nil {
		return err
	}

	for id, library := range pass.libraries {
		if _, used := inUse[id]; used || !library.Metadata().Finalizers().Has(diskControllerName) {
			continue
		}

		if err := r.RemoveFinalizer(ctx, library.Metadata(), diskControllerName); err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to release content library %q: %w", id, err)
		}

		logger.Info("released content library held for a virtual machine disk", zap.String("content_library", id))
	}

	return nil
}

// librariesInUse names the libraries which must stay held, by ID: the ones held names, plus every
// library named by a disk status something else holds.
func librariesInUse(ctx context.Context, reader controller.Reader, held map[string]struct{}) (map[string]struct{}, error) {
	diskStatuses, err := safe.ReaderListAll[*hypervisor.VirtualMachineDiskStatus](ctx, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to list virtual machine disk statuses: %w", err)
	}

	inUse := maps.Clone(held)

	for diskStatus := range diskStatuses.All() {
		if diskStatus.Metadata().Finalizers().Empty() {
			continue
		}

		if library := diskStatus.TypedSpec().Image.Library; library != "" {
			inUse[library] = struct{}{}
		}
	}

	return inUse, nil
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

	image := disk.Provision.FromImage

	library, found := pass.libraries[image.Library]
	if !found {
		return hypervisor.VirtualMachineDiskStatusSpec{
			Phase: hypervisor.VirtualMachineDiskPhaseNotReady,
		}, fmt.Errorf("content library %q is not configured", image.Library)
	}

	if err := holdLibrary(ctx, r, logger, library); err != nil {
		return hypervisor.VirtualMachineDiskStatusSpec{
			Phase: hypervisor.VirtualMachineDiskPhaseNotReady,
		}, err
	}

	pass.held[image.Library] = struct{}{}

	return disks.ObserveImage(disk, *library.TypedSpec())
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
		if !disk.Provision.Blank {
			// Copying or backing a disk from a content library image is not implemented yet; see
			// the follow-up to siderolabs/talos#14511.
			return fmt.Errorf("%w: a disk is only provisioned from provision.blank today", errDiskUnsupported)
		}

		return checkBlankDiskSupported(disk)
	default:
		return fmt.Errorf("%w: unsupported type %q", errDiskUnsupported, disk.Type)
	}
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
