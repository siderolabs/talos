// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package disks_test

import (
	"os"
	"path/filepath"
	"testing"

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

	require.NoError(t, os.WriteFile(filepath.Join(path, ".stage.upload"), []byte("image"), 0o600))

	hash := digest.SHA256.FromString("image").String()
	require.NoError(t, contentlibrary.Publish(root, ".stage.upload", "image.iso", hash))

	disk := hypervisor.VirtualMachineDiskSpec{}
	disk.Provision.FromImage = &hypervisor.VirtualMachineDiskFromImageSpec{Library: "images", File: "image.iso"}
	got, err := disks.ObserveImage(disk, hypervisor.ContentLibraryStatusSpec{Phase: hypervisor.ContentLibraryPhaseReady, Path: path})
	require.NoError(t, err)
	require.Equal(t, filepath.Join(path, "objects", "sha256", digest.SHA256.FromString("image").Encoded()), got.SourcePath)
}
