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

	"github.com/siderolabs/talos/internal/pkg/contentlibrary"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// rawDiskFormat is libvirt's driver type for a file attached as-is, which is how a cdrom's image is
// presented. A blank disk takes its driver type from the volume that was made for it.
const rawDiskFormat = "raw"

// ObserveImage finds the provisioning source within a library held until its pool copy is ready.
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

	object, err := checkLibraryFile(library.Path, image.File, image.Digest)
	if err != nil {
		return hypervisor.VirtualMachineDiskStatusSpec{
			Phase: hypervisor.VirtualMachineDiskPhaseNotReady,
		}, fmt.Errorf("content library %q: file %q: %w", image.Library, image.File, err)
	}

	return hypervisor.VirtualMachineDiskStatusSpec{
		// Storage copies this source; domains attach only the independent pool destination.
		SourcePath: filepath.Join(library.Path, object),
		Format:     rawDiskFormat,
		ReadOnly:   true,
		Phase:      hypervisor.VirtualMachineDiskPhaseReady,
	}, nil
}

// checkLibraryFile confirms the image exists and, when a digest is pinned, that it still hashes to it.
func checkLibraryFile(libraryPath, name, expected string) (string, error) {
	root, err := os.OpenRoot(libraryPath)
	if err != nil {
		return "", fmt.Errorf("failed to open content library directory: %w", err)
	}

	defer root.Close() //nolint:errcheck

	object, err := contentlibrary.Resolve(root, name)
	if err != nil {
		return "", err
	}

	f, err := root.Open(object)
	if err != nil {
		return "", err
	}

	defer f.Close() //nolint:errcheck

	info, err := f.Stat()
	if err != nil {
		return "", err
	}

	if !info.Mode().IsRegular() {
		return "", errors.New("not a regular file")
	}

	if expected == "" {
		return object, nil
	}

	return object, VerifyDigest(f, expected)
}

// VerifyDigest hashes the complete reader and checks it against the expected digest.
// Image observation uses it to validate a pinned provisioning source before copying.
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
