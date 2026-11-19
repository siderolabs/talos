// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumeconfig

import (
	"os"
	"path/filepath"
	"strconv"

	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

const inlineDataVolumeTmpfsInodesParameter = "nr_inodes"

// InlineDataVolumeTmpfsSize returns the size of the tmpfs backing an inline data volume.
//
// It fits two full sets of files, as an update writes the new set before removing the old.
//
// The config validation rounds each file up to constants.InlineDataVolumeBlockSize, but the tmpfs
// allocates whole pages of the node's page size, which might be larger (e.g. 64 KiB on some arm64 kernels):
// each file then takes at most a page minus a block more than validated.
// The headroom covers the (bounded) directories of two sets of files, and the symlinks, at a page each.
//
// The size is only a limit for the tmpfs, so a larger page size doesn't reserve more memory.
func InlineDataVolumeTmpfsSize() int {
	return InlineDataVolumeTmpfsSizeForPageSize(os.Getpagesize())
}

// InlineDataVolumeTmpfsSizeForPageSize returns the size of the tmpfs backing an inline data volume for the given page size.
func InlineDataVolumeTmpfsSizeForPageSize(pageSize int) int {
	perFileOverhead := max(0, pageSize-constants.InlineDataVolumeBlockSize)
	perSet := constants.InlineDataVolumeMaxContentsSize + constants.InlineDataVolumeMaxFiles*perFileOverhead

	return 2*perSet + 2*constants.InlineDataVolumeMaxDirectories*pageSize
}

// InlineDataVolumeTmpfsInodes is the number of inodes of the tmpfs backing an inline data volume.
//
// It fits the worst case: two sets of files, each with its `..data-<hash>` directory, files and directories,
// the top-level symbolic links (at most one per file), and the root, `..data` and temporary links.
const InlineDataVolumeTmpfsInodes = 2*(1+constants.InlineDataVolumeMaxFiles+constants.InlineDataVolumeMaxDirectories) +
	constants.InlineDataVolumeMaxFiles + 16

// InlineDataVolumeTransformer is the transformer for inline data volume configs.
func InlineDataVolumeTransformer(c configconfig.Config) ([]VolumeResource, error) {
	if c == nil {
		return nil, nil
	}

	resources := make([]VolumeResource, 0, len(c.InlineDataVolumeConfigs()))

	for _, inlineDataVolumeConfig := range c.InlineDataVolumeConfigs() {
		resources = append(resources, VolumeResource{
			VolumeID: constants.InlineDataVolumePrefix + inlineDataVolumeConfig.Name(),
			Label:    block.InlineDataVolumeLabel,
			TransformFunc: NewBuilder().
				WithType(block.VolumeTypeTmpfs).
				WithMount(block.MountSpec{
					TargetPath:   filepath.Join(constants.InlineDataVolumeMountPoint, constants.InlineDataVolumePrefix+inlineDataVolumeConfig.Name()),
					SelinuxLabel: constants.EphemeralSelinuxLabel,
					FileMode:     0o755,
					UID:          0,
					GID:          0,
					Parameters: []block.ParameterSpec{
						block.NewStringParameter(block.TmpfsSizeParameter, strconv.Itoa(InlineDataVolumeTmpfsSize())),
						block.NewStringParameter(inlineDataVolumeTmpfsInodesParameter, strconv.Itoa(InlineDataVolumeTmpfsInodes)),
					},
				}).
				WriterFunc(),
			MountTransformFunc: HandleInlineDataVolumeMountRequest,
		})
	}

	return resources, nil
}

// HandleInlineDataVolumeMountRequest is the MountTransformFunc for inline data volumes.
func HandleInlineDataVolumeMountRequest(m *block.VolumeMountRequest) error {
	m.TypedSpec().Detached = true
	m.TypedSpec().ReadOnly = false // machined needs read-write to update contents
	m.TypedSpec().DisableAccessTime = true
	m.TypedSpec().Secure = true
	m.TypedSpec().NoExec = true

	return nil
}
