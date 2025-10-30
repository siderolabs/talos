// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumeconfig_test

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/volumes/volumeconfig"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

var baseCfg v1alpha1.Config

func init() {
	u, _ := url.Parse("https://foo:6443") //nolint:errcheck

	baseCfg = v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{},
		ClusterConfig: &v1alpha1.ClusterConfig{
			ControlPlane: &v1alpha1.ControlPlaneConfig{
				Endpoint: &v1alpha1.Endpoint{URL: u},
			},
		},
	}
}

func TestGetSystemVolumeTransformers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	encryptionMeta := runtime.NewMetaKey(runtime.NamespaceName, runtime.MetaKeyTagToID(meta.StateEncryptionConfig))

	transformers := volumeconfig.GetSystemVolumeTransformers(ctx, encryptionMeta, false, false)
	require.Len(t, transformers, 5, "should return 5 transformers")

	var allResources []volumeconfig.VolumeResource

	for _, transformer := range transformers {
		resources, err := transformer(container.NewV1Alpha1(&baseCfg))
		require.NoError(t, err)

		allResources = append(allResources, resources...)
	}

	for _, volumeID := range []string{
		constants.MetaPartitionLabel,
		constants.StatePartitionLabel,
		constants.EphemeralPartitionLabel,
		"/var/run",
		"/var/log",
		"/etc/cni",
	} {
		assert.Condition(t, func() bool {
			for _, r := range allResources {
				if r.VolumeID == volumeID {
					return true
				}
			}

			return false
		}, "should have volume config resource for %s", volumeID)
	}
}

func TestGetStateVolumeTransformer(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string

		encryptionMeta *runtime.MetaKey
		inContainer    bool
		isAgent        bool
		cfg            *v1alpha1.Config

		checkFunc func(t *testing.T, resources []volumeconfig.VolumeResource)
	}{
		{
			name:           "in container",
			inContainer:    true,
			isAgent:        false,
			encryptionMeta: nil,
			cfg:            &baseCfg,
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) {
				require.Len(t, resources, 1)

				testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
					require.NoError(t, err)

					assert.Equal(t, block.VolumeTypeDirectory, vc.TypedSpec().Type)
					assert.Equal(t, constants.StateMountPoint, vc.TypedSpec().Mount.TargetPath)
				})
			},
		},
		{
			name:           "W/ config",
			inContainer:    false,
			isAgent:        false,
			encryptionMeta: nil,
			cfg:            &baseCfg,
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) { //nolint:dupl
				require.Len(t, resources, 1)

				testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
					require.NoError(t, err)

					assert.Equal(t, block.VolumeTypePartition, vc.TypedSpec().Type)
					assert.Equal(t, constants.StateMountPoint, vc.TypedSpec().Mount.TargetPath)

					assert.NotEmpty(t, vc.TypedSpec().Provisioning)
				})
			},
		},
		{
			name:           "NO config",
			inContainer:    false,
			isAgent:        false,
			encryptionMeta: nil,
			cfg:            nil,
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) {
				require.Len(t, resources, 1)

				testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
					require.NoError(t, err)

					assert.Equal(t, block.VolumeTypePartition, vc.TypedSpec().Type)
					assert.Equal(t, constants.StateMountPoint, vc.TypedSpec().Mount.TargetPath)

					locator, err := vc.TypedSpec().Locator.Match.MarshalText()
					require.NoError(t, err)
					assert.Equal(t, `volume.partition_label == "STATE" && volume.name != ""`, string(locator))
				})
			},
		},
		{
			name:           "agent w/ NO config = no match locator",
			inContainer:    false,
			isAgent:        true,
			encryptionMeta: nil,
			cfg:            nil,
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) {
				require.Len(t, resources, 1)

				testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
					require.NoError(t, err)

					locator, err := vc.TypedSpec().Locator.Match.MarshalText()
					require.NoError(t, err)
					assert.Equal(t, "false", string(locator))
				})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			transformer := volumeconfig.GetStateVolumeTransformer(tc.encryptionMeta, tc.inContainer, tc.isAgent)
			resources, err := transformer(container.NewV1Alpha1(tc.cfg))
			require.NoError(t, err)

			assert.Equal(t, constants.StatePartitionLabel, resources[0].VolumeID)
			assert.Equal(t, block.SystemVolumeLabel, resources[0].Label)

			tc.checkFunc(t, resources)
		})
	}
}

func TestGetEphemeralVolumeTransformer(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string

		inContainer bool
		cfg         *v1alpha1.Config

		checkFunc func(t *testing.T, resources []volumeconfig.VolumeResource)
	}{
		{
			name:        "NO config",
			inContainer: false,
			cfg:         nil,
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) {
				require.Len(t, resources, 0)
			},
		},
		{
			name:        "container W/ config",
			inContainer: true,
			cfg:         &baseCfg,
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) { //nolint:dupl
				require.Len(t, resources, 1)

				testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
					require.NoError(t, err)

					assert.Equal(t, block.VolumeTypeDirectory, vc.TypedSpec().Type)
					assert.Equal(t, constants.EphemeralMountPoint, vc.TypedSpec().Mount.TargetPath)

					assert.Empty(t, vc.TypedSpec().Provisioning)
				})
			},
		},
		{
			name:        "W/ config",
			inContainer: false,
			cfg:         &baseCfg,
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) {
				require.Len(t, resources, 1)

				assert.Equal(t, constants.EphemeralPartitionLabel, resources[0].VolumeID)

				testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
					require.NoError(t, err)

					assert.Equal(t, block.VolumeTypePartition, vc.TypedSpec().Type)
					assert.Equal(t, constants.EphemeralMountPoint, vc.TypedSpec().Mount.TargetPath)

					assert.NotEmpty(t, vc.TypedSpec().Provisioning)
				})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			transformer := volumeconfig.GetEphemeralVolumeTransformer(tc.inContainer)
			resources, err := transformer(container.NewV1Alpha1(tc.cfg))
			require.NoError(t, err)

			tc.checkFunc(t, resources)
		})
	}
}

func TestStandardDirectoryVolumesTransformer(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string

		cfg *v1alpha1.Config

		checkFunc func(t *testing.T, resources []volumeconfig.VolumeResource)
	}{
		{
			name: "NO config",
			cfg:  nil,
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) {
				require.Len(t, resources, 0)
			},
		},
		{
			name: "W/ config",
			cfg:  &baseCfg,
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) {
				require.Len(t, resources, 1+14) // +1 for /var/run symlink, +14 for standard directories

				var varRunSymlinkResource *volumeconfig.VolumeResource
				for i := range resources {
					if resources[i].VolumeID == "/var/run" {
						varRunSymlinkResource = &resources[i]

						break
					}
				}

				require.NotNil(t, varRunSymlinkResource, "should have /var/run symlink resource")
				assert.Equal(t, block.SystemVolumeLabel, varRunSymlinkResource.Label)
				testTransformFunc(t, varRunSymlinkResource.TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
					require.NoError(t, err)

					assert.Equal(t, block.VolumeTypeSymlink, vc.TypedSpec().Type)
					assert.Equal(t, "/run", vc.TypedSpec().Symlink.SymlinkTargetPath)
				})

				// Check some standard directories
				expectedPaths := []string{"/var/log", "/var/lib", constants.EtcdDataVolumeID}
				for _, expectedPath := range expectedPaths {
					var found bool
					for i := range resources {
						if resources[i].VolumeID == expectedPath {
							found = true
							assert.Equal(t, block.SystemVolumeLabel, resources[i].Label)
							testTransformFunc(t, resources[i].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
								require.NoError(t, err)

								assert.Equal(t, block.VolumeTypeDirectory, vc.TypedSpec().Type)
							})

							break
						}
					}
					assert.True(t, found, "should have resource for %s", expectedPath)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resources, err := volumeconfig.StandardDirectoryVolumesTransformer(container.NewV1Alpha1(tc.cfg))
			require.NoError(t, err)

			tc.checkFunc(t, resources)
		})
	}
}

func TestGetOverlayVolumesTransformer(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string

		inContainer bool
		cfg         *v1alpha1.Config
		checkFunc   func(t *testing.T, resources []volumeconfig.VolumeResource)
	}{
		{
			name:        "NO config",
			inContainer: false,
			cfg:         nil,
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) {
				require.Len(t, resources, 0)
			},
		},
		{
			name:        "in container",
			inContainer: true,
			cfg:         &baseCfg,
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) {
				require.Len(t, resources, 0)
			},
		},
		{
			name:        "W/ config",
			inContainer: false,
			cfg:         &baseCfg,
			checkFunc: func(t *testing.T, resources []volumeconfig.VolumeResource) {
				require.Len(t, resources, len(constants.Overlays))

				for _, resource := range resources {
					assert.Equal(t, block.SystemVolumeLabel, resource.Label)

					testTransformFunc(t, resource.TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
						require.NoError(t, err)

						assert.Equal(t, block.VolumeTypeOverlay, vc.TypedSpec().Type)
						assert.Equal(t, constants.EphemeralPartitionLabel, vc.TypedSpec().ParentID)
					})
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resources, err := volumeconfig.GetOverlayVolumesTransformer(tc.inContainer)(container.NewV1Alpha1(tc.cfg))
			require.NoError(t, err)

			tc.checkFunc(t, resources)
		})
	}
}

func TestStateVolumeTransformerWithEncryptionMeta(t *testing.T) {
	t.Parallel()

	// Create encryption meta
	encryptionConfig := &v1alpha1.EncryptionConfig{
		EncryptionProvider: "luks2",
		EncryptionKeys: []*v1alpha1.EncryptionKey{
			{
				KeySlot: 1,
				KeyStatic: &v1alpha1.EncryptionKeyStatic{
					KeyData: "test-secret",
				},
			},
		},
	}

	encryptionMetaMarshalled, err := json.Marshal(encryptionConfig)
	require.NoError(t, err)

	encryptionMeta := runtime.NewMetaKey(runtime.NamespaceName, runtime.MetaKeyTagToID(meta.StateEncryptionConfig))
	encryptionMeta.TypedSpec().Value = string(encryptionMetaMarshalled)

	transformer := volumeconfig.GetStateVolumeTransformer(encryptionMeta, false, false)

	resources, err := transformer(nil)
	require.NoError(t, err)
	require.Len(t, resources, 1)

	testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
		require.NoError(t, err)

		// Encryption should be set from meta
		assert.NotEmpty(t, vc.TypedSpec().Encryption)
	})
}

func TestEphemeralVolumeTransformerWithExtraConfig(t *testing.T) {
	t.Parallel()

	ephemeralConfig := blockcfg.NewVolumeConfigV1Alpha1()
	ephemeralConfig.MetaName = constants.EphemeralPartitionLabel
	ephemeralConfig.ProvisioningSpec.ProvisioningMinSize = blockcfg.MustByteSize("10GiB")
	ephemeralConfig.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize("100GiB")

	cfg, err := container.New(
		baseCfg.DeepCopy(),
		ephemeralConfig,
	)
	require.NoError(t, err)

	transformer := volumeconfig.GetEphemeralVolumeTransformer(false)
	resources, err := transformer(cfg)
	require.NoError(t, err)

	testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
		require.NoError(t, err)

		require.Len(t, resources, 1)

		assert.EqualValues(t, 10*1024*1024*1024, vc.TypedSpec().Provisioning.PartitionSpec.MinSize)
		assert.EqualValues(t, 100*1024*1024*1024, vc.TypedSpec().Provisioning.PartitionSpec.MaxSize)
	})
}

func testTransformFunc(t *testing.T,
	transformer func(vc *block.VolumeConfig) error,
	checkFunc func(t *testing.T, vc *block.VolumeConfig, err error),
) {
	t.Helper()

	vc := block.NewVolumeConfig(block.NamespaceName, "test")
	err := transformer(vc)

	checkFunc(t, vc, err)
}
