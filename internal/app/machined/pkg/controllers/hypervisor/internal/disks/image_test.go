// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package disks_test

import (
	"crypto/sha512"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/disks"
	"github.com/siderolabs/talos/internal/pkg/contentlibrary"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func TestObserveImagePinsObjectInsteadOfMutableAlias(t *testing.T) {
	path := t.TempDir()
	root, err := os.OpenRoot(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })
	publishImage(t, root, path, "image.iso", "image")

	disk := hypervisor.VirtualMachineDiskSpec{}
	disk.Provision.FromImage = &hypervisor.VirtualMachineDiskFromImageSpec{
		Library: "images",
		File:    "image.iso",
	}
	verifier := disks.ImageVerifier{}
	got, err := verifier.Observe(disk, hypervisor.ContentLibraryStatusSpec{
		Phase: hypervisor.ContentLibraryPhaseReady,
		Path:  path,
	})
	require.NoError(t, err)
	require.Equal(t, filepath.Join(path, "objects", "sha256", digest.SHA256.FromString("image").Encoded()), got.SourcePath)
}

func TestImageVerifierWarmAndColdPins(t *testing.T) {
	path := t.TempDir()
	root, err := os.OpenRoot(path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })
	publishImage(t, root, path, "image.iso", "image")

	pin := sha512.Sum512([]byte("image"))
	disk := hypervisor.VirtualMachineDiskSpec{}
	disk.Provision.FromImage = &hypervisor.VirtualMachineDiskFromImageSpec{
		Library: "images",
		File:    "image.iso",
		Digest:  digest.NewDigestFromBytes(digest.SHA512, pin[:]).String(),
	}
	library := hypervisor.ContentLibraryStatusSpec{
		Phase: hypervisor.ContentLibraryPhaseReady,
		Path:  path,
	}
	verifier := disks.ImageVerifier{}
	first, err := verifier.Observe(disk, library)
	require.NoError(t, err)
	warm, err := verifier.Observe(disk, library)
	require.NoError(t, err)
	require.Equal(t, first, warm)

	info, err := os.Stat(first.SourcePath)
	require.NoError(t, err)

	changed := info.ModTime().Add(time.Second)
	require.NoError(t, os.Chtimes(first.SourcePath, changed, changed))

	_, err = verifier.Observe(disk, library)
	require.NoError(t, err)

	cold := disks.ImageVerifier{}
	_, err = cold.Observe(disk, library)
	require.NoError(t, err)

	require.NoError(t, root.Remove("image.iso"))
	publishImage(t, root, path, "image.iso", "replacement")

	_, err = verifier.Observe(disk, library)
	require.ErrorIs(t, err, disks.ErrImageDigestMismatch, "an alias replacement must not reuse the previous object's verification")
}

func publishImage(t *testing.T, root *os.Root, path, name, contents string) {
	t.Helper()

	stage, err := os.CreateTemp(path, ".stage-*.upload")
	require.NoError(t, err)
	_, err = stage.WriteString(contents)
	require.NoError(t, err)
	require.NoError(t, stage.Sync())
	info, err := stage.Stat()
	require.NoError(t, err)
	require.NoError(t, stage.Close())
	require.NoError(t, contentlibrary.Publish(root, filepath.Base(stage.Name()), name, digest.SHA256.FromString(contents).String(), info))
}

func TestSourceResolutionCategories(t *testing.T) {
	for _, test := range []struct {
		name, file string
		phase      hypervisor.ContentLibraryPhase
		want       error
	}{
		{"invalid alias", "../image.iso", hypervisor.ContentLibraryPhaseReady, disks.ErrSourceRejected},
		{"missing alias", "missing.iso", hypervisor.ContentLibraryPhaseReady, disks.ErrSourceWaiting},
		{"library not ready", "image.iso", hypervisor.ContentLibraryPhaseNotReady, disks.ErrSourceWaiting},
	} {
		t.Run(test.name, func(t *testing.T) {
			verifier := disks.ImageVerifier{}
			disk := hypervisor.VirtualMachineDiskSpec{Provision: hypervisor.VirtualMachineDiskProvisionSpec{FromImage: &hypervisor.VirtualMachineDiskFromImageSpec{Library: "images", File: test.file}}}
			_, err := verifier.Observe(disk, hypervisor.ContentLibraryStatusSpec{Path: t.TempDir(), Phase: test.phase})
			require.ErrorIs(t, err, test.want)
			require.NotErrorIs(t, err, disks.ErrSourceRetryable)
		})
	}
}
