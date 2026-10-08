// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/opencontainers/go-digest"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// rawDiskFormat is libvirt's driver type for a file attached as-is.
const rawDiskFormat = "raw"

const diskControllerName = "hypervisor.VirtualMachineDiskController"

// errHoldFailed marks the controller's own failure to hold a library.
var errHoldFailed = errors.New("failed to hold content library")

// VirtualMachineDiskController resolves each disk of a virtual machine to a host source.
type VirtualMachineDiskController struct{}

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
			// Strong: an image is attached where it lies, so a library's mount has to outlive every guest reading one.
			Kind: controller.InputStrong,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDiskStatusType,
			Kind:      controller.InputWeak,
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

	wanted := make(map[resource.ID]struct{}, specs.Len())
	held := map[string]struct{}{}

	var errs []error

	for vm := range specs.All() {
		name := vm.Metadata().ID()

		for _, disk := range vm.TypedSpec().Disks {
			id := hypervisor.VirtualMachineDiskStatusID(name, disk)
			wanted[id] = struct{}{}

			if err := ctrl.reconcileDisk(ctx, r, logger, name, id, disk, libraries, held); err != nil {
				errs = append(errs, err)
			}
		}
	}

	// A status the configuration has dropped is torn down to ask VirtualMachineController for its hold back.
	if err := cleanupOutputs[*hypervisor.VirtualMachineDiskStatus](ctx, r, "virtual machine disk status", wanted); err != nil {
		return errors.Join(append(errs, err)...)
	}

	return errors.Join(append(errs, ctrl.releaseLibraries(ctx, r, logger, libraries, held))...)
}

// reconcileDisk publishes the status of one disk of one virtual machine.
func (ctrl *VirtualMachineDiskController) reconcileDisk(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	name string,
	id resource.ID,
	disk hypervisor.VirtualMachineDiskSpec,
	libraries map[string]*hypervisor.ContentLibraryStatus,
	held map[string]struct{},
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
	resolved, resolveErr := ctrl.resolve(ctx, r, logger, disk, libraries, held)
	if errors.Is(resolveErr, errHoldFailed) {
		return fmt.Errorf("failed to resolve virtual machine disk %q: %w", id, resolveErr)
	}

	if err := safe.WriterModify(ctx, r,
		hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, id),
		func(res *hypervisor.VirtualMachineDiskStatus) error {
			*res.TypedSpec() = resolved
			res.TypedSpec().VirtualMachine = name
			res.TypedSpec().Name = disk.Name

			// Stamped outside the resolution, so a failed one is still attributed to the image it was for.
			if image := disk.Provision.FromImage; image != nil {
				res.TypedSpec().Image = *image
			}

			if resolveErr != nil {
				res.TypedSpec().Error = resolveErr.Error()
			}

			return nil
		},
	); err != nil {
		return fmt.Errorf("failed to write virtual machine disk status %q: %w", id, err)
	}

	return nil
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
	libraries map[string]*hypervisor.ContentLibraryStatus, held map[string]struct{},
) error {
	inUse, err := librariesInUse(ctx, r, held)
	if err != nil {
		return err
	}

	for id, library := range libraries {
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

// resolve finds the host source for a disk, holding the library it resolved against first and
// recording it in held so the same pass does not give it straight back.
func (ctrl *VirtualMachineDiskController) resolve(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	disk hypervisor.VirtualMachineDiskSpec,
	libraries map[string]*hypervisor.ContentLibraryStatus,
	held map[string]struct{},
) (hypervisor.VirtualMachineDiskStatusSpec, error) {
	if err := checkVirtualMachineDiskSupported(disk); err != nil {
		return hypervisor.VirtualMachineDiskStatusSpec{}, err
	}

	// An empty drive resolves to no host source at all, so it holds no library: the guest is shown
	// a cdrom with its tray open. Checked before the library lookup, which has nothing to look up.
	if disk.Provision.FromImage == nil {
		return emptyDriveStatus(), nil
	}

	image := disk.Provision.FromImage

	library, found := libraries[image.Library]
	if !found {
		return hypervisor.VirtualMachineDiskStatusSpec{}, fmt.Errorf("content library %q is not configured", image.Library)
	}

	if err := holdLibrary(ctx, r, logger, library); err != nil {
		return hypervisor.VirtualMachineDiskStatusSpec{}, err
	}

	held[image.Library] = struct{}{}

	return resolveVirtualMachineDisk(disk, *library.TypedSpec())
}

// checkVirtualMachineDiskSupported reports whether a disk is one this slice provisions at all, as
// opposed to one that is merely not resolved yet. Kept apart so the status and the render grade it alike.
func checkVirtualMachineDiskSupported(disk hypervisor.VirtualMachineDiskSpec) error {
	switch {
	case disk.Type != hypervisorhelpers.VirtualMachineDiskTypeCDROM.String():
		return fmt.Errorf("%w: only %s disks are provisioned today, this one is %q",
			errDiskUnsupported, hypervisorhelpers.VirtualMachineDiskTypeCDROM, disk.Type)
	case disk.Provision.FromImage == nil && !disk.Provision.Blank:
		// Machine configuration validation already requires one of the two of a cdrom; check anyway,
		// so the status is the whole truth about a disk rather than a partial one.
		return fmt.Errorf("%w: a cdrom requires provision.fromImage or provision.blank", errDiskUnsupported)
	}

	return nil
}

// emptyDriveStatus is a cdrom with no medium in it.
//
// Format and ReadOnly match what a loaded drive resolves to, and they have to: the definition of a
// loaded and an empty drive differ only in the source element, which is what lets the medium be
// changed on a running domain instead of restarting it.
func emptyDriveStatus() hypervisor.VirtualMachineDiskStatusSpec {
	return hypervisor.VirtualMachineDiskStatusSpec{
		Format:   rawDiskFormat,
		ReadOnly: true,
		Ready:    true,
	}
}

// resolveVirtualMachineDisk finds the host source for a disk within a library already held for it.
func resolveVirtualMachineDisk(
	disk hypervisor.VirtualMachineDiskSpec,
	library hypervisor.ContentLibraryStatusSpec,
) (hypervisor.VirtualMachineDiskStatusSpec, error) {
	if disk.Provision.FromImage == nil {
		return emptyDriveStatus(), nil
	}

	image := disk.Provision.FromImage

	if !library.Ready {
		return hypervisor.VirtualMachineDiskStatusSpec{}, fmt.Errorf("content library %q is not ready: %s", image.Library, library.Error)
	}

	if err := checkLibraryFile(library.Path, image.File, image.Digest); err != nil {
		return hypervisor.VirtualMachineDiskStatusSpec{}, fmt.Errorf("content library %q: file %q: %w", image.Library, image.File, err)
	}

	return hypervisor.VirtualMachineDiskStatusSpec{
		// Attached where it lies: nothing copies the image, so the library's mount has to stay under it.
		SourcePath: filepath.Join(library.Path, image.File),
		Format:     rawDiskFormat,
		ReadOnly:   true,
		Ready:      true,
	}, nil
}

// checkLibraryFile confirms the image exists and, when a digest is pinned, that it still hashes to it.
func checkLibraryFile(libraryPath, name, expected string) error {
	root, err := os.OpenRoot(libraryPath)
	if err != nil {
		return fmt.Errorf("failed to open content library directory: %w", err)
	}

	defer root.Close() //nolint:errcheck

	f, err := root.Open(name)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	info, err := f.Stat()
	if err != nil {
		return err
	}

	if !info.Mode().IsRegular() {
		return errors.New("not a regular file")
	}

	if expected == "" {
		return nil
	}

	return verifyDigest(f, expected)
}

// verifyDigest rehashes the whole file. It runs on every reconciliation: a library file is not
// immutable, and nothing else notices when it changes underneath a virtual machine.
func verifyDigest(r io.Reader, expected string) error {
	// Machine configuration validation already accepted this digest, including its algorithm.
	dgst, err := digest.Parse(expected)
	if err != nil {
		return fmt.Errorf("digest %q is invalid: %w", expected, err)
	}

	verifier := dgst.Verifier()

	if _, err := io.Copy(verifier, r); err != nil {
		return fmt.Errorf("failed to read for digest verification: %w", err)
	}

	if !verifier.Verified() {
		return fmt.Errorf("digest mismatch: expected %s", dgst)
	}

	return nil
}
