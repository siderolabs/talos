// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package disks constructs backing-resource requests and observes virtual machine disk sources.
// Resource ownership, finalizers and status publication remain with the controller.
package disks

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/opencontainers/go-digest"

	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// rawDiskFormat is libvirt's driver type for a file attached as-is, which is how a cdrom's image is
// presented. A blank disk takes its driver type from the volume that was made for it.
const rawDiskFormat = "raw"

// ObserveImage finds the host source within a library already held by the controller.
// The disk must have passed the controller's supported-disk validation.
func ObserveImage(
	disk hypervisor.VirtualMachineDiskSpec,
	library hypervisor.ContentLibraryStatusSpec,
) (hypervisor.VirtualMachineDiskStatusSpec, error) {
	image := disk.Provision.FromImage

	if library.Phase != hypervisor.ContentLibraryPhaseReady {
		return hypervisor.VirtualMachineDiskStatusSpec{
			Phase: hypervisor.VirtualMachineDiskPhaseNotReady,
		}, fmt.Errorf("content library %q is not ready: %s", image.Library, library.Error)
	}

	if err := checkLibraryFile(library.Path, image.File, image.Digest); err != nil {
		return hypervisor.VirtualMachineDiskStatusSpec{
			Phase: hypervisor.VirtualMachineDiskPhaseNotReady,
		}, fmt.Errorf("content library %q: file %q: %w", image.Library, image.File, err)
	}

	return hypervisor.VirtualMachineDiskStatusSpec{
		// Attached where it lies: nothing copies the image, so the library's mount has to stay under it.
		SourcePath: filepath.Join(library.Path, image.File),
		Format:     rawDiskFormat,
		ReadOnly:   true,
		Phase:      hypervisor.VirtualMachineDiskPhaseReady,
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

	return VerifyDigest(f, expected)
}

// VerifyDigest rehashes the whole file. Image observation calls it on every reconciliation:
// a library file is not immutable, and nothing else notices when it changes underneath a
// virtual machine. The domain controller also uses it after acquiring its disk holds.
func VerifyDigest(r io.Reader, expected string) error {
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
