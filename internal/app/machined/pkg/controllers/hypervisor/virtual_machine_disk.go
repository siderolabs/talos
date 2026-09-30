// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/opencontainers/go-digest"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// rawDiskFormat is libvirt's driver type for a file attached as-is.
const rawDiskFormat = "raw"

// VirtualMachineDiskController resolves each disk of a virtual machine to a host source.
type VirtualMachineDiskController struct{}

// Name implements controller.Controller interface.
func (ctrl *VirtualMachineDiskController) Name() string {
	return "hypervisor.VirtualMachineDiskController"
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
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.VirtualMachineDiskStatusType,
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
	}
}

// Run implements controller.Controller interface.
func (ctrl *VirtualMachineDiskController) Run(ctx context.Context, runtime controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-runtime.EventCh():
		}

		if err := ctrl.reconcile(ctx, runtime); err != nil {
			return err
		}

		runtime.ResetRestartBackoff()
	}
}

func (ctrl *VirtualMachineDiskController) reconcile(ctx context.Context, runtime controller.Runtime) error {
	specs, err := safe.ReaderListAll[*hypervisor.VirtualMachineSpec](ctx, runtime)
	if err != nil {
		return fmt.Errorf("failed to list virtual machine specs: %w", err)
	}

	libraryStatuses, err := safe.ReaderListAll[*hypervisor.ContentLibraryStatus](ctx, runtime)
	if err != nil {
		return fmt.Errorf("failed to list content library statuses: %w", err)
	}

	libraries := make(map[string]hypervisor.ContentLibraryStatusSpec, libraryStatuses.Len())

	for library := range libraryStatuses.All() {
		libraries[library.Metadata().ID()] = *library.TypedSpec()
	}

	runtime.StartTrackingOutputs()

	var errs []error

	for vm := range specs.All() {
		name := vm.Metadata().ID()

		for _, disk := range vm.TypedSpec().Disks {
			id := hypervisor.VirtualMachineDiskStatusID(name, disk.Name)

			resolved, resolveErr := resolveVirtualMachineDisk(disk, libraries)

			if err := safe.WriterModify(ctx, runtime,
				hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, id),
				func(res *hypervisor.VirtualMachineDiskStatus) error {
					*res.TypedSpec() = resolved
					res.TypedSpec().VirtualMachine = name
					res.TypedSpec().Name = disk.Name

					// Stamped outside the resolution, so a failed one is still attributed to the
					// image it was for: a status which does not name the image the configuration
					// asks for today is stale, whether it is ready or not.
					if image := disk.Provision.FromImage; image != nil {
						res.TypedSpec().Image = *image
					}

					if resolveErr != nil {
						res.TypedSpec().Error = resolveErr.Error()
					}

					return nil
				},
			); err != nil {
				errs = append(errs, fmt.Errorf("failed to write virtual machine disk status %q: %w", id, err))
			}
		}
	}

	return errors.Join(append(errs,
		safe.CleanupOutputs[*hypervisor.VirtualMachineDiskStatus](ctx, runtime))...)
}

// checkVirtualMachineDiskSupported reports whether a disk is one this slice provisions at all, as
// opposed to one that is merely not resolved yet.
//
// Kept apart from the rest of the resolution so both the disk's own status and the domain render
// grade it the same way: nothing that happens on the host turns an unsupported disk into a usable
// one, so it must not be reported as something to wait for.
func checkVirtualMachineDiskSupported(disk hypervisor.VirtualMachineDiskSpec) error {
	switch {
	case disk.Type != hypervisorhelpers.VirtualMachineDiskTypeCDROM.String():
		return fmt.Errorf("%w: only %s disks are provisioned today, this one is %q",
			errDiskUnsupported, hypervisorhelpers.VirtualMachineDiskTypeCDROM, disk.Type)
	case disk.Provision.FromImage == nil:
		// Machine configuration validation already requires this of a cdrom; check anyway, so the
		// status is the whole truth about a disk rather than a partial one.
		return fmt.Errorf("%w: a cdrom requires provision.fromImage", errDiskUnsupported)
	}

	return nil
}

// resolveVirtualMachineDisk finds the host source for a disk.
func resolveVirtualMachineDisk(
	disk hypervisor.VirtualMachineDiskSpec,
	libraries map[string]hypervisor.ContentLibraryStatusSpec,
) (hypervisor.VirtualMachineDiskStatusSpec, error) {
	if err := checkVirtualMachineDiskSupported(disk); err != nil {
		return hypervisor.VirtualMachineDiskStatusSpec{}, err
	}

	image := disk.Provision.FromImage

	library, found := libraries[image.Library]
	if !found {
		return hypervisor.VirtualMachineDiskStatusSpec{}, fmt.Errorf("content library %q is not configured", image.Library)
	}

	if !library.Ready {
		return hypervisor.VirtualMachineDiskStatusSpec{}, fmt.Errorf("content library %q is not ready: %s", image.Library, library.Error)
	}

	if err := checkLibraryFile(library.Path, image.File, image.Digest); err != nil {
		return hypervisor.VirtualMachineDiskStatusSpec{}, fmt.Errorf("content library %q: file %q: %w", image.Library, image.File, err)
	}

	return hypervisor.VirtualMachineDiskStatusSpec{
		// The image is attached where it lies: nothing copies it, so the library file is pinned
		// for as long as the virtual machine refers to it.
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
