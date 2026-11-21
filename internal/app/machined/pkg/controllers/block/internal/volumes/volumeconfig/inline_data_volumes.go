// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumeconfig

import (
	"path/filepath"
	"strconv"

	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

const inlineDataVolumeTmpfsInodesParameter = "nr_inodes"

// InlineDataVolumeTmpfsSize fits two full sets of files, as an update writes the new set before removing the old.
const InlineDataVolumeTmpfsSize = 2*constants.InlineDataVolumeMaxContentsSize + inlineDataVolumeTmpfsHeadroom

// inlineDataVolumeTmpfsHeadroom covers the directories of two sets of files, and the symlinks.
const inlineDataVolumeTmpfsHeadroom = 2 * constants.InlineDataVolumeMaxFiles * constants.InlineDataVolumePageSize

// InlineDataVolumeTmpfsInodes is the number of inodes of the tmpfs backing an inline data volume.
const InlineDataVolumeTmpfsInodes = 5*constants.InlineDataVolumeMaxFiles + 16

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
						block.NewStringParameter(block.TmpfsSizeParameter, strconv.Itoa(InlineDataVolumeTmpfsSize)),
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
	m.TypedSpec().ReadOnly = false
	m.TypedSpec().DisableAccessTime = true
	m.TypedSpec().Secure = true
	m.TypedSpec().NoExec = true

	return nil
}
