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

// ErrImageDigestMismatch distinguishes an incompatible source from an absent library.
var ErrImageDigestMismatch = errors.New("image digest mismatch")

// ErrSourceWaiting requires an input change before resolution can progress.
var ErrSourceWaiting = errors.New("waiting for image source")

// ErrSourceRetryable identifies an operational source failure needing a timed retry.
var ErrSourceRetryable = errors.New("retryable image source failure")

// ErrSourceRejected identifies invalid or incompatible source intent.
var ErrSourceRejected = errors.New("rejected image source")

// ImageVerifier caches successful pins for unchanged immutable object/file identities.
// Its zero value is ready for use by one controller run.
type ImageVerifier struct {
	verified map[string]imageVerification
}

type imageVerification struct {
	object string
	file   os.FileInfo
}

// Observe finds the provisioning source within a library held until its pool copy is ready.
// The disk must have passed the controller's supported-disk validation.
func (verifier *ImageVerifier) Observe(
	disk hypervisor.VirtualMachineDiskSpec,
	library hypervisor.ContentLibraryStatusSpec,
) (hypervisor.VirtualMachineDiskStatusSpec, error) {
	image := disk.Provision.FromImage

	if library.Phase != hypervisor.ContentLibraryPhaseReady {
		return hypervisor.VirtualMachineDiskStatusSpec{
			Phase: hypervisor.VirtualMachineDiskPhaseNotReady,
		}, fmt.Errorf("%w: content library %q is not ready: %s", ErrSourceWaiting, image.Library, library.Error)
	}

	object, err := verifier.checkLibraryFile(library.Path, image.File, image.Digest)
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
func (verifier *ImageVerifier) checkLibraryFile(libraryPath, name, expected string) (string, error) {
	root, err := os.OpenRoot(libraryPath)
	if err != nil {
		return "", fmt.Errorf("%w: failed to open content library directory: %w", ErrSourceRetryable, err)
	}

	defer root.Close() //nolint:errcheck

	object, err := contentlibrary.Resolve(root, name)
	if err != nil {
		var pathErr *os.PathError
		if !errors.As(err, &pathErr) {
			return "", fmt.Errorf("%w: %w", ErrSourceRejected, err)
		}

		if errors.Is(err, os.ErrNotExist) && pathErr.Path == name {
			return "", fmt.Errorf("%w: %w", ErrSourceWaiting, err)
		}

		return "", classifySourceError(err)
	}

	f, err := root.Open(object)
	if err != nil {
		return "", classifySourceError(err)
	}

	defer f.Close() //nolint:errcheck

	info, err := f.Stat()
	if err != nil {
		return "", classifySourceError(err)
	}

	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%w: not a regular file", ErrSourceRejected)
	}

	if expected == "" {
		return object, nil
	}

	key := filepath.Join(libraryPath, name) + "\x00" + expected

	return object, classifySourceError(verifier.verifyObject(f, info, key, object, expected))
}

func (verifier *ImageVerifier) verifyObject(f *os.File, info os.FileInfo, key, object, expected string) error {
	if previous, ok := verifier.verified[key]; ok && previous.object == object && sameImageFile(previous.file, info) {
		return nil
	}

	if err := VerifyDigest(f, expected); err != nil {
		return err
	}

	after, err := f.Stat()
	if err != nil {
		return err
	}

	if !sameImageFile(info, after) {
		return errors.New("image source changed during digest verification")
	}

	if verifier.verified == nil {
		verifier.verified = make(map[string]imageVerification)
	}

	verifier.verified[key] = imageVerification{
		object: object,
		file:   after,
	}

	return nil
}

func sameImageFile(a, b os.FileInfo) bool {
	return os.SameFile(a, b) && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

// VerifyDigest hashes the complete reader and checks it against the expected digest.
// Image observation uses it to validate a pinned provisioning source before copying.
func VerifyDigest(r io.Reader, expected string) error {
	// Machine configuration validation already accepted this digest, including its algorithm.
	dgst, err := digest.Parse(expected)
	if err != nil {
		return fmt.Errorf("%w: digest %q is invalid: %w", ErrSourceRejected, expected, err)
	}

	verifier := dgst.Verifier()

	if _, err := io.Copy(verifier, r); err != nil {
		return fmt.Errorf("failed to read for digest verification: %w", err)
	}

	if !verifier.Verified() {
		return fmt.Errorf("%w: expected %s", ErrImageDigestMismatch, dgst)
	}

	return nil
}

func classifySourceError(err error) error {
	if err == nil {
		return nil
	}

	category := ErrSourceRetryable
	if errors.Is(err, ErrImageDigestMismatch) || errors.Is(err, ErrSourceRejected) {
		category = ErrSourceRejected
	}

	return fmt.Errorf("%w: %w", category, err)
}
