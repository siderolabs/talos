// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumeconfig_test

import (
	"io/fs"
	"testing"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/volumes/volumeconfig"
	"github.com/siderolabs/talos/internal/pkg/partition"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

//nolint:dupl
func TestUserVolumeTransformer(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		cfg       []*blockcfg.UserVolumeConfigV1Alpha1
		checkFunc func(t *testing.T, resources []volumeconfig.VolumeResource, err error)
	}{
		{
			name: "no config",
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource, err error) {
				require.Len(t, resources, 0)
				require.NoError(t, err)
			},
		},
		{
			name: "partition volume",
			cfg: []*blockcfg.UserVolumeConfigV1Alpha1{
				{
					Meta: meta.Meta{
						MetaKind:       blockcfg.UserVolumeConfigKind,
						MetaAPIVersion: "v1alpha1",
					},
					MetaName:   "foo",
					VolumeType: pointer.To(block.VolumeTypePartition),
					FilesystemSpec: blockcfg.FilesystemSpec{
						FilesystemType: block.FilesystemTypeXFS,
					},
				},
			},
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource, err error) {
				require.NoError(t, err)
				require.Len(t, resources, 1)

				assert.Equal(t, constants.UserVolumePrefix+"foo", resources[0].VolumeID)
				assert.Equal(t, block.UserVolumeLabel, resources[0].Label)

				testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
					require.NoError(t, err)

					assert.Equal(t, block.VolumeTypePartition, vc.TypedSpec().Type)

					assert.Equal(t, block.FilesystemTypeXFS, vc.TypedSpec().Provisioning.FilesystemSpec.Type)

					assert.Equal(t, "foo", vc.TypedSpec().Mount.TargetPath)
					assert.Equal(t, constants.UserVolumeMountPoint, vc.TypedSpec().Mount.ParentID)
					assert.Equal(t, fs.FileMode(0o755), vc.TypedSpec().Mount.FileMode)
				})

				testMountTransformFunc(t, resources[0].MountTransformFunc, func(t *testing.T, m *block.VolumeMountRequest, err error) {
					// default mount transform is noop
					require.NoError(t, err)
				})
			},
		},
		{
			name: "directory volume",
			cfg: []*blockcfg.UserVolumeConfigV1Alpha1{{
				Meta: meta.Meta{
					MetaKind:       blockcfg.UserVolumeConfigKind,
					MetaAPIVersion: "v1alpha1",
				},
				MetaName:   "bar",
				VolumeType: pointer.To(block.VolumeTypeDirectory),
				FilesystemSpec: blockcfg.FilesystemSpec{
					FilesystemType: block.FilesystemTypeXFS,
				},
			}},
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource, err error) {
				require.NoError(t, err)
				require.Len(t, resources, 1)

				assert.Equal(t, constants.UserVolumePrefix+"bar", resources[0].VolumeID)
				assert.Equal(t, block.UserVolumeLabel, resources[0].Label)

				testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
					require.NoError(t, err)

					assert.Equal(t, block.VolumeTypeDirectory, vc.TypedSpec().Type)

					require.Empty(t, vc.TypedSpec().Provisioning)

					assert.Equal(t, "bar", vc.TypedSpec().Mount.TargetPath)
					assert.Equal(t, constants.UserVolumeMountPoint, vc.TypedSpec().Mount.ParentID)
					assert.Equal(t, pointer.To("bar"), vc.TypedSpec().Mount.BindTarget)
					assert.Equal(t, fs.FileMode(0o755), vc.TypedSpec().Mount.FileMode)
				})

				testMountTransformFunc(t, resources[0].MountTransformFunc, func(t *testing.T, m *block.VolumeMountRequest, err error) {
					// default mount transform is noop
					require.NoError(t, err)
				})
			},
		},
		{
			name: "unsupported volume type",
			cfg: []*blockcfg.UserVolumeConfigV1Alpha1{{
				Meta: meta.Meta{
					MetaKind:       blockcfg.UserVolumeConfigKind,
					MetaAPIVersion: "v1alpha1",
				},
				VolumeType: pointer.To(block.VolumeTypeTmpfs),
			}},
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource, err error) {
				require.Error(t, err)
				assert.Equal(t, "unsupported volume type \"tmpfs\"", err.Error())

				require.Empty(t, resources)
			},
		},
		{
			name: "multiple configs",
			cfg: []*blockcfg.UserVolumeConfigV1Alpha1{{
				Meta: meta.Meta{
					MetaKind:       blockcfg.UserVolumeConfigKind,
					MetaAPIVersion: "v1alpha1",
				},
				MetaName:   "foo",
				VolumeType: pointer.To(block.VolumeTypePartition),
				FilesystemSpec: blockcfg.FilesystemSpec{
					FilesystemType: block.FilesystemTypeXFS,
				},
			}, {
				Meta: meta.Meta{
					MetaKind:       blockcfg.UserVolumeConfigKind,
					MetaAPIVersion: "v1alpha1",
				},
				MetaName:   "bar",
				VolumeType: pointer.To(block.VolumeTypeDirectory),
				FilesystemSpec: blockcfg.FilesystemSpec{
					FilesystemType: block.FilesystemTypeXFS,
				},
			}},
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource, err error) {
				require.NoError(t, err)
				require.Len(t, resources, 2)

				assert.Equal(t, constants.UserVolumePrefix+"foo", resources[0].VolumeID)
				assert.Equal(t, block.UserVolumeLabel, resources[0].Label)

				assert.Equal(t, constants.UserVolumePrefix+"bar", resources[1].VolumeID)
				assert.Equal(t, block.UserVolumeLabel, resources[1].Label)

				testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
					require.NoError(t, err)

					assert.Equal(t, block.VolumeTypePartition, vc.TypedSpec().Type)
					assert.Equal(t, block.FilesystemTypeXFS, vc.TypedSpec().Provisioning.FilesystemSpec.Type)
				})

				testTransformFunc(t, resources[1].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
					require.NoError(t, err)

					assert.Equal(t, block.VolumeTypeDirectory, vc.TypedSpec().Type)
				})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mergedCfg, err := container.New(xslices.Map(tc.cfg,
				func(cfg *blockcfg.UserVolumeConfigV1Alpha1) configconfig.Document {
					return cfg
				})...)
			require.NoError(t, err)

			resources, err := volumeconfig.UserVolumeTransformer(mergedCfg)

			tc.checkFunc(t, resources, err)
		})
	}
}

//nolint:dupl
func TestRawVolumeTransformer(t *testing.T) {
	t.Parallel()

	volumeCfg := &blockcfg.RawVolumeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       blockcfg.RawVolumeConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: "raw-data",
	}

	cfg, err := container.New(volumeCfg)
	require.NoError(t, err)

	resources, err := volumeconfig.RawVolumeTransformer(cfg)
	require.NoError(t, err)

	assert.Equal(t, block.RawVolumeLabel, resources[0].Label)
	require.Len(t, resources, 1)

	assert.Equal(t, constants.RawVolumePrefix+"raw-data", resources[0].VolumeID)
	assert.Equal(t, block.RawVolumeLabel, resources[0].Label)

	testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
		require.NoError(t, err)

		assert.Equal(t, block.VolumeTypePartition, vc.TypedSpec().Type)

		assert.Equal(t, block.WaveUserVolumes, vc.TypedSpec().Provisioning.Wave)
		assert.Equal(t, block.FilesystemTypeNone, vc.TypedSpec().Provisioning.FilesystemSpec.Type)
		assert.Equal(t, constants.RawVolumePrefix+"raw-data", vc.TypedSpec().Provisioning.PartitionSpec.Label)
		assert.Equal(t, partition.LinuxFilesystemData, vc.TypedSpec().Provisioning.PartitionSpec.TypeUUID)
		assert.Equal(t, block.FilesystemTypeNone, vc.TypedSpec().Provisioning.FilesystemSpec.Type)
	})

	testMountTransformFunc(t, resources[0].MountTransformFunc, func(t *testing.T, m *block.VolumeMountRequest, err error) {
		// SkipMountTransform should return an error tagged with SkipUserVolumeMountRequest
		require.Error(t, err)
		assert.Equal(t, "skip", err.Error())
	})
}

//nolint:dupl
func TestExistingVolumeTransformer(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		cfg       []*blockcfg.ExistingVolumeConfigV1Alpha1
		checkFunc func(t *testing.T, resources []volumeconfig.VolumeResource)
	}{
		{
			name: "no config",
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) {
				require.Len(t, resources, 0)
			},
		},
		{
			name: "existing volume RW",
			cfg: []*blockcfg.ExistingVolumeConfigV1Alpha1{
				{
					Meta: meta.Meta{
						MetaKind:       blockcfg.ExistingVolumeConfigKind,
						MetaAPIVersion: "v1alpha1",
					},
					MetaName: "existing-data",
					VolumeDiscoverySpec: blockcfg.VolumeDiscoverySpec{
						VolumeSelectorConfig: blockcfg.VolumeSelector{
							Match: cel.MustExpression(cel.ParseBooleanExpression(`volume.partition_label == "MY-DATA"`, celenv.VolumeLocator())),
						},
					},
					MountSpec: blockcfg.ExistingMountSpec{
						MountReadOnly: pointer.To(false),
					},
				},
			},
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) {
				require.Len(t, resources, 1)

				assert.Equal(t, block.ExistingVolumeLabel, resources[0].Label)
				assert.Equal(t, constants.ExistingVolumePrefix+"existing-data", resources[0].VolumeID)

				testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
					require.NoError(t, err)

					assert.Equal(t, block.VolumeTypePartition, vc.TypedSpec().Type)

					assert.Equal(t, "existing-data", vc.TypedSpec().Mount.TargetPath)
					assert.Equal(t, constants.UserVolumeMountPoint, vc.TypedSpec().Mount.ParentID)
					assert.Equal(t, fs.FileMode(0o755), vc.TypedSpec().Mount.FileMode)
				})

				testMountTransformFunc(t, resources[0].MountTransformFunc, func(t *testing.T, m *block.VolumeMountRequest, err error) {
					require.NoError(t, err)

					assert.False(t, m.TypedSpec().ReadOnly, "expected read-write mount")
				})
			},
		},
		{
			name: "existing volume RO",
			cfg: []*blockcfg.ExistingVolumeConfigV1Alpha1{
				{
					Meta: meta.Meta{
						MetaKind:       blockcfg.ExistingVolumeConfigKind,
						MetaAPIVersion: "v1alpha1",
					},
					MetaName: "readonly-data",
					VolumeDiscoverySpec: blockcfg.VolumeDiscoverySpec{
						VolumeSelectorConfig: blockcfg.VolumeSelector{
							Match: cel.MustExpression(cel.ParseBooleanExpression(`volume.partition_label == "READONLY-DATA"`, celenv.VolumeLocator())),
						},
					},
					MountSpec: blockcfg.ExistingMountSpec{
						MountReadOnly: pointer.To(true),
					},
				},
			},
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) {
				require.Len(t, resources, 1)

				testMountTransformFunc(t, resources[0].MountTransformFunc, func(t *testing.T, m *block.VolumeMountRequest, err error) {
					require.NoError(t, err)

					assert.True(t, m.TypedSpec().ReadOnly, "expected read-only mount")
				})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mergedCfg, err := container.New(xslices.Map(tc.cfg,
				func(cfg *blockcfg.ExistingVolumeConfigV1Alpha1) configconfig.Document {
					return cfg
				})...)
			require.NoError(t, err)

			resources, err := volumeconfig.ExistingVolumeTransformer(mergedCfg)
			require.NoError(t, err)

			tc.checkFunc(t, resources)
		})
	}
}

//nolint:dupl
func TestExternalVolumeTransformer(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		cfg       []*blockcfg.ExternalVolumeConfigV1Alpha1
		checkFunc func(t *testing.T, resources []volumeconfig.VolumeResource)
	}{
		{
			name: "no config",
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) {
				require.Len(t, resources, 0)
			},
		},
		{
			name: "external volume RW",
			cfg: []*blockcfg.ExternalVolumeConfigV1Alpha1{
				{
					Meta: meta.Meta{
						MetaKind:       blockcfg.ExternalVolumeConfigKind,
						MetaAPIVersion: "v1alpha1",
					},
					MetaName:       "external-data",
					FilesystemType: block.FilesystemTypeVirtiofs,
					MountSpec: blockcfg.ExternalMountSpec{
						MountReadOnly: pointer.To(false),
						MountVirtiofs: &blockcfg.VirtiofsMountSpec{
							VirtiofsTag: "data",
						},
					},
				},
			},
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) {
				require.Len(t, resources, 1)

				assert.Equal(t, block.ExternalVolumeLabel, resources[0].Label)
				assert.Equal(t, constants.ExternalVolumePrefix+"external-data", resources[0].VolumeID)

				testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
					require.NoError(t, err)

					assert.Equal(t, block.VolumeTypeExternal, vc.TypedSpec().Type)

					assert.Equal(t, "external-data", vc.TypedSpec().Mount.TargetPath)
					assert.Equal(t, constants.UserVolumeMountPoint, vc.TypedSpec().Mount.ParentID)
					assert.Equal(t, fs.FileMode(0o755), vc.TypedSpec().Mount.FileMode)
					assert.Equal(t, "data", vc.TypedSpec().Provisioning.DiskSelector.External)
				})

				testMountTransformFunc(t, resources[0].MountTransformFunc, func(t *testing.T, m *block.VolumeMountRequest, err error) {
					require.NoError(t, err)

					assert.False(t, m.TypedSpec().ReadOnly, "expected read-write mount")
				})
			},
		},
		{
			name: "external volume RW",
			cfg: []*blockcfg.ExternalVolumeConfigV1Alpha1{
				{
					Meta: meta.Meta{
						MetaKind:       blockcfg.ExternalVolumeConfigKind,
						MetaAPIVersion: "v1alpha1",
					},
					MetaName:       "external-data",
					FilesystemType: block.FilesystemTypeVirtiofs,
					MountSpec: blockcfg.ExternalMountSpec{
						MountReadOnly: pointer.To(true),
						MountVirtiofs: &blockcfg.VirtiofsMountSpec{
							VirtiofsTag: "data",
						},
					},
				},
			},
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) {
				require.Len(t, resources, 1)

				assert.Equal(t, block.ExternalVolumeLabel, resources[0].Label)
				assert.Equal(t, constants.ExternalVolumePrefix+"external-data", resources[0].VolumeID)

				testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
					require.NoError(t, err)

					assert.Equal(t, block.VolumeTypeExternal, vc.TypedSpec().Type)

					assert.Equal(t, "external-data", vc.TypedSpec().Mount.TargetPath)
					assert.Equal(t, constants.UserVolumeMountPoint, vc.TypedSpec().Mount.ParentID)
					assert.Equal(t, fs.FileMode(0o755), vc.TypedSpec().Mount.FileMode)
					assert.Equal(t, "data", vc.TypedSpec().Provisioning.DiskSelector.External)
				})

				testMountTransformFunc(t, resources[0].MountTransformFunc, func(t *testing.T, m *block.VolumeMountRequest, err error) {
					require.NoError(t, err)

					assert.True(t, m.TypedSpec().ReadOnly, "expected read-write mount")
				})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mergedCfg, err := container.New(xslices.Map(tc.cfg,
				func(cfg *blockcfg.ExternalVolumeConfigV1Alpha1) configconfig.Document {
					return cfg
				})...)
			require.NoError(t, err)

			resources, err := volumeconfig.ExternalVolumeTransformer(mergedCfg)
			require.NoError(t, err)

			tc.checkFunc(t, resources)
		})
	}
}

//nolint:dupl
func TestSwapVolumeTransformer(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		cfg       []*blockcfg.SwapVolumeConfigV1Alpha1
		checkFunc func(t *testing.T, resources []volumeconfig.VolumeResource)
	}{
		{
			name: "no config",
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) {
				require.Len(t, resources, 0)
			},
		},
		{
			name: "swap volume",
			cfg: []*blockcfg.SwapVolumeConfigV1Alpha1{
				{
					Meta: meta.Meta{
						MetaKind:       blockcfg.SwapVolumeConfigKind,
						MetaAPIVersion: "v1alpha1",
					},
					MetaName: "swap1",
				},
			},
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) {
				require.Len(t, resources, 1)

				assert.Equal(t, constants.SwapVolumePrefix+"swap1", resources[0].VolumeID)
				assert.Equal(t, block.SwapVolumeLabel, resources[0].Label)

				testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
					require.NoError(t, err)

					assert.Equal(t, block.VolumeTypePartition, vc.TypedSpec().Type)

					assert.Equal(t, block.FilesystemTypeSwap, vc.TypedSpec().Provisioning.FilesystemSpec.Type)
					assert.Equal(t, constants.SwapVolumePrefix+"swap1", vc.TypedSpec().Provisioning.PartitionSpec.Label)
					assert.Equal(t, block.WaveUserVolumes, vc.TypedSpec().Provisioning.Wave)

					assert.EqualValues(t, volumeconfig.MinUserVolumeSize, vc.TypedSpec().Provisioning.PartitionSpec.MinSize)
					assert.EqualValues(t, 0, vc.TypedSpec().Provisioning.PartitionSpec.MaxSize)
				})

				testMountTransformFunc(t, resources[0].MountTransformFunc, func(t *testing.T, m *block.VolumeMountRequest, err error) {
					// default mount transform is noop
					require.NoError(t, err)
				})
			},
		},
		{
			name: "swap volume with sizes",
			cfg: []*blockcfg.SwapVolumeConfigV1Alpha1{
				{
					Meta: meta.Meta{
						MetaKind:       blockcfg.SwapVolumeConfigKind,
						MetaAPIVersion: "v1alpha1",
					},
					MetaName: "swap1",
					ProvisioningSpec: blockcfg.ProvisioningSpec{
						ProvisioningMinSize: blockcfg.MustByteSize("1GB"),
						ProvisioningMaxSize: blockcfg.MustSize("2GB"),
					},
				},
			},
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) {
				require.Len(t, resources, 1)

				assert.Equal(t, constants.SwapVolumePrefix+"swap1", resources[0].VolumeID)
				assert.Equal(t, block.SwapVolumeLabel, resources[0].Label)

				testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
					require.NoError(t, err)

					assert.Equal(t, block.VolumeTypePartition, vc.TypedSpec().Type)

					assert.Equal(t, block.FilesystemTypeSwap, vc.TypedSpec().Provisioning.FilesystemSpec.Type)
					assert.Equal(t, constants.SwapVolumePrefix+"swap1", vc.TypedSpec().Provisioning.PartitionSpec.Label)
					assert.Equal(t, block.WaveUserVolumes, vc.TypedSpec().Provisioning.Wave)

					assert.EqualValues(t, 1*1000*1000*1000, vc.TypedSpec().Provisioning.PartitionSpec.MinSize)
					assert.EqualValues(t, 2*1000*1000*1000, vc.TypedSpec().Provisioning.PartitionSpec.MaxSize)
				})

				testMountTransformFunc(t, resources[0].MountTransformFunc, func(t *testing.T, m *block.VolumeMountRequest, err error) {
					// default mount transform is noop
					require.NoError(t, err)
				})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mergedCfg, err := container.New(xslices.Map(tc.cfg,
				func(cfg *blockcfg.SwapVolumeConfigV1Alpha1) configconfig.Document {
					return cfg
				})...)
			require.NoError(t, err)

			resources, err := volumeconfig.SwapVolumeTransformer(mergedCfg)
			require.NoError(t, err)

			tc.checkFunc(t, resources)
		})
	}
}

func testMountTransformFunc(t *testing.T,
	transformer func(*block.VolumeMountRequest) error,
	checkFunc func(t *testing.T, m *block.VolumeMountRequest, err error),
) {
	t.Helper()

	m := block.NewVolumeMountRequest(block.NamespaceName, "test")
	err := transformer(m)

	checkFunc(t, m, err)
}
