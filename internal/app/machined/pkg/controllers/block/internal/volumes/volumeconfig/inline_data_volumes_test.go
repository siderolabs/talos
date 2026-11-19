// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumeconfig_test

import (
	"io/fs"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/volumes/volumeconfig"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func newInlineDataVolumeDoc(name string) *blockcfg.InlineDataVolumeConfigV1Alpha1 {
	doc := blockcfg.NewInlineDataVolumeConfigV1Alpha1()
	doc.MetaName = name
	doc.FilesConfig = map[string]blockcfg.InlineDataVolumeFileV1Alpha1{
		"app.yaml": {FileContents: "a: b\n"},
	}

	return doc
}

func TestInlineDataVolumeTransformer(t *testing.T) {
	t.Parallel()

	resources, err := volumeconfig.InlineDataVolumeTransformer(nil)
	require.NoError(t, err)
	require.Empty(t, resources)

	userVolume := blockcfg.NewUserVolumeConfigV1Alpha1()
	userVolume.MetaName = "data"
	userVolume.VolumeType = new(block.VolumeTypeDirectory)

	cfg, err := container.New(
		newInlineDataVolumeDoc("app-config"),
		newInlineDataVolumeDoc("other"),
		userVolume,
	)
	require.NoError(t, err)

	resources, err = volumeconfig.InlineDataVolumeTransformer(cfg)
	require.NoError(t, err)

	ids := make([]string, 0, len(resources))
	for _, resource := range resources {
		ids = append(ids, resource.VolumeID)

		assert.Equal(t, block.InlineDataVolumeLabel, resource.Label)
	}

	assert.Equal(t, []string{"i-app-config", "i-other"}, ids)

	for _, resource := range resources {
		testTransformFunc(t, resource.TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
			require.NoError(t, err)

			assert.Equal(t, block.VolumeTypeTmpfs, vc.TypedSpec().Type)
			assert.Equal(t, filepath.Join(constants.InlineDataVolumeMountPoint, resource.VolumeID), vc.TypedSpec().Mount.TargetPath)
			assert.Empty(t, vc.TypedSpec().Mount.ParentID)
			assert.Equal(t, constants.EphemeralSelinuxLabel, vc.TypedSpec().Mount.SelinuxLabel)
			assert.Equal(t, fs.FileMode(0o755), vc.TypedSpec().Mount.FileMode)
			assert.Equal(t,
				[]block.ParameterSpec{
					block.NewStringParameter(block.TmpfsSizeParameter, strconv.Itoa(volumeconfig.InlineDataVolumeTmpfsSize())),
					block.NewStringParameter("nr_inodes", strconv.Itoa(volumeconfig.InlineDataVolumeTmpfsInodes)),
				},
				vc.TypedSpec().Mount.Parameters,
			)
			require.NoError(t, block.ValidateTmpfsParameters(vc.TypedSpec().Mount.Parameters))

			assert.Empty(t, vc.Metadata().Labels().Raw())
		})

		testMountTransformFunc(t, resource.MountTransformFunc, func(t *testing.T, m *block.VolumeMountRequest, err error) {
			require.NoError(t, err)

			assert.True(t, m.TypedSpec().Detached, "only machined writes to the volume, through the file descriptor")
			assert.False(t, m.TypedSpec().ReadOnly)
			assert.True(t, m.TypedSpec().Secure)
			assert.True(t, m.TypedSpec().NoExec)
			assert.True(t, m.TypedSpec().DisableAccessTime)
		})
	}
}

func TestInlineDataVolumeTmpfsSize(t *testing.T) {
	t.Parallel()

	assert.GreaterOrEqual(t, volumeconfig.InlineDataVolumeTmpfsSize(), 2*constants.InlineDataVolumeMaxContentsSize)
	assert.GreaterOrEqual(t, volumeconfig.InlineDataVolumeTmpfsInodes, 4*constants.InlineDataVolumeMaxFiles)

	// the validation rounds files up to a fixed block size, the tmpfs to the node's page size:
	// two sets of the most files allowed, each a full block, must fit on any page size
	for _, pageSize := range []int{4096, 16384, 65536} {
		t.Run(strconv.Itoa(pageSize), func(t *testing.T) {
			t.Parallel()

			size := volumeconfig.InlineDataVolumeTmpfsSizeForPageSize(pageSize)

			assert.GreaterOrEqual(t, size, 2*constants.InlineDataVolumeMaxContentsSize)
			assert.GreaterOrEqual(t, size, 2*constants.InlineDataVolumeMaxFiles*pageSize)
		})
	}
}
