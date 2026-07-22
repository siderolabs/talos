// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumeconfig_test

import (
	"context"
	"encoding/json"
	"fmt"
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
	require.Len(t, transformers, 6, "should return 6 transformers")

	var allResources []volumeconfig.VolumeResource //nolint:prealloc // this is a test

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
		constants.LogVolumeID,
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
				require.Len(t, resources, 1+10) // +1 for /var/run symlink, +10 for standard directories (ETCD/CRI/KUBELET/LOG are promotable, handled separately)

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

				// Check some standard directories (/var/log itself is promotable; its children remain here)
				expectedPaths := []string{"/var/log/audit", "/var/lib", "/var/lib/cni"}
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

func TestGetPromotableSystemVolumesTransformer(t *testing.T) {
	t.Parallel()

	findResource := func(t *testing.T, resources []volumeconfig.VolumeResource, volumeID string) volumeconfig.VolumeResource {
		t.Helper()

		for i := range resources {
			if resources[i].VolumeID == volumeID {
				return resources[i]
			}
		}

		t.Fatalf("missing resource for %q", volumeID)

		return volumeconfig.VolumeResource{}
	}

	t.Run("default is directory", func(t *testing.T) {
		t.Parallel()

		transformer := volumeconfig.GetPromotableSystemVolumesTransformer(false)
		resources, err := transformer(container.NewV1Alpha1(&baseCfg))
		require.NoError(t, err)
		require.Len(t, resources, 4)

		for _, volumeID := range []string{constants.EtcdDataVolumeID, constants.CRIContainerdVolumeID, constants.KubeletDataVolumeID, constants.LogVolumeID} {
			r := findResource(t, resources, volumeID)

			testTransformFunc(t, r.TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
				require.NoError(t, err)

				assert.Equal(t, block.VolumeTypeDirectory, vc.TypedSpec().Type)
				assert.Empty(t, vc.TypedSpec().Provisioning)
			})
		}
	})

	t.Run("promoted to partition via VolumeConfig", func(t *testing.T) {
		t.Parallel()

		etcdCfg := blockcfg.NewVolumeConfigV1Alpha1()
		etcdCfg.MetaName = constants.EtcdDataVolumeID
		etcdCfg.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize("50GiB")

		cfg, err := container.New(baseCfg.DeepCopy(), etcdCfg)
		require.NoError(t, err)

		transformer := volumeconfig.GetPromotableSystemVolumesTransformer(false)
		resources, err := transformer(cfg)
		require.NoError(t, err)

		// ETCD promoted to a partition
		testTransformFunc(t, findResource(t, resources, constants.EtcdDataVolumeID).TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
			require.NoError(t, err)

			assert.Equal(t, block.VolumeTypePartition, vc.TypedSpec().Type)
			assert.Equal(t, block.WaveSystemDisk, vc.TypedSpec().Provisioning.Wave)
			assert.Equal(t, constants.EtcdDataVolumeID, vc.TypedSpec().Provisioning.PartitionSpec.Label)
			assert.EqualValues(t, 50*1024*1024*1024, vc.TypedSpec().Provisioning.PartitionSpec.MaxSize)

			locator, err := vc.TypedSpec().Locator.Match.MarshalText()
			require.NoError(t, err)
			assert.Equal(t, `volume.partition_label == "ETCD"`, string(locator))
		})

		// KUBELET stays a directory
		testTransformFunc(t, findResource(t, resources, constants.KubeletDataVolumeID).TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
			require.NoError(t, err)

			assert.Equal(t, block.VolumeTypeDirectory, vc.TypedSpec().Type)
		})
	})

	// promoteCfg builds a config that promotes a single volume via a maxSize provisioning request.
	promoteCfg := func(t *testing.T, volumeID string) *container.Container {
		t.Helper()

		volCfg := blockcfg.NewVolumeConfigV1Alpha1()
		volCfg.MetaName = volumeID
		volCfg.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize("50GiB")

		cfg, err := container.New(baseCfg.DeepCopy(), volCfg)
		require.NoError(t, err)

		return cfg
	}

	// assertPromoted asserts that volumeID is provisioned as a dedicated partition.
	assertPromoted := func(t *testing.T, resources []volumeconfig.VolumeResource, volumeID string) {
		t.Helper()

		testTransformFunc(t, findResource(t, resources, volumeID).TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
			require.NoError(t, err)

			assert.Equal(t, block.VolumeTypePartition, vc.TypedSpec().Type)
			assert.Equal(t, block.WaveSystemDisk, vc.TypedSpec().Provisioning.Wave)
			assert.Equal(t, volumeID, vc.TypedSpec().Provisioning.PartitionSpec.Label)
			assert.Equal(t, block.FilesystemTypeXFS, vc.TypedSpec().Provisioning.FilesystemSpec.Type)

			locator, err := vc.TypedSpec().Locator.Match.MarshalText()
			require.NoError(t, err)
			assert.Equal(t, `volume.partition_label == "`+volumeID+`"`, string(locator))
		})
	}

	assertDirectory := func(t *testing.T, resources []volumeconfig.VolumeResource, volumeID string) {
		t.Helper()

		testTransformFunc(t, findResource(t, resources, volumeID).TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
			require.NoError(t, err)

			assert.Equal(t, block.VolumeTypeDirectory, vc.TypedSpec().Type)
		})
	}

	t.Run("promote CRI", func(t *testing.T) {
		t.Parallel()

		transformer := volumeconfig.GetPromotableSystemVolumesTransformer(false)
		resources, err := transformer(promoteCfg(t, constants.CRIContainerdVolumeID))
		require.NoError(t, err)

		assertPromoted(t, resources, constants.CRIContainerdVolumeID)
		assertDirectory(t, resources, constants.EtcdDataVolumeID)
		assertDirectory(t, resources, constants.KubeletDataVolumeID)
	})

	t.Run("promote KUBELET", func(t *testing.T) {
		t.Parallel()

		transformer := volumeconfig.GetPromotableSystemVolumesTransformer(false)
		resources, err := transformer(promoteCfg(t, constants.KubeletDataVolumeID))
		require.NoError(t, err)

		assertPromoted(t, resources, constants.KubeletDataVolumeID)
		assertDirectory(t, resources, constants.EtcdDataVolumeID)
		assertDirectory(t, resources, constants.CRIContainerdVolumeID)
	})

	t.Run("promote LOG", func(t *testing.T) {
		t.Parallel()

		transformer := volumeconfig.GetPromotableSystemVolumesTransformer(false)
		resources, err := transformer(promoteCfg(t, constants.LogVolumeID))
		require.NoError(t, err)

		assertPromoted(t, resources, constants.LogVolumeID)
		assertDirectory(t, resources, constants.EtcdDataVolumeID)
		assertDirectory(t, resources, constants.CRIContainerdVolumeID)
		assertDirectory(t, resources, constants.KubeletDataVolumeID)
	})

	t.Run("promote all four", func(t *testing.T) {
		t.Parallel()

		etcdCfg := blockcfg.NewVolumeConfigV1Alpha1()
		etcdCfg.MetaName = constants.EtcdDataVolumeID
		etcdCfg.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize("50GiB")

		criCfg := blockcfg.NewVolumeConfigV1Alpha1()
		criCfg.MetaName = constants.CRIContainerdVolumeID
		criCfg.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize("50GiB")

		kubeletCfg := blockcfg.NewVolumeConfigV1Alpha1()
		kubeletCfg.MetaName = constants.KubeletDataVolumeID
		kubeletCfg.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize("50GiB")

		logCfg := blockcfg.NewVolumeConfigV1Alpha1()
		logCfg.MetaName = constants.LogVolumeID
		logCfg.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize("50GiB")

		cfg, err := container.New(baseCfg.DeepCopy(), etcdCfg, criCfg, kubeletCfg, logCfg)
		require.NoError(t, err)

		transformer := volumeconfig.GetPromotableSystemVolumesTransformer(false)
		resources, err := transformer(cfg)
		require.NoError(t, err)

		assertPromoted(t, resources, constants.EtcdDataVolumeID)
		assertPromoted(t, resources, constants.CRIContainerdVolumeID)
		assertPromoted(t, resources, constants.KubeletDataVolumeID)
		assertPromoted(t, resources, constants.LogVolumeID)
	})

	t.Run("promoted mount is secure by default", func(t *testing.T) {
		t.Parallel()

		transformer := volumeconfig.GetPromotableSystemVolumesTransformer(false)
		resources, err := transformer(promoteCfg(t, constants.LogVolumeID))
		require.NoError(t, err)

		testTransformFunc(t, findResource(t, resources, constants.LogVolumeID).TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
			require.NoError(t, err)

			assert.Equal(t, block.VolumeTypePartition, vc.TypedSpec().Type)
			assert.True(t, vc.TypedSpec().Mount.Secure, "promoted volume is secure by default (like EPHEMERAL)")
		})
	})

	t.Run("promoted mount honors mount.secure=false", func(t *testing.T) {
		t.Parallel()

		secureOff := false
		logCfg := blockcfg.NewVolumeConfigV1Alpha1()
		logCfg.MetaName = constants.LogVolumeID
		logCfg.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize("50GiB")
		logCfg.MountSpec.MountSecure = &secureOff

		cfg, err := container.New(baseCfg.DeepCopy(), logCfg)
		require.NoError(t, err)

		transformer := volumeconfig.GetPromotableSystemVolumesTransformer(false)
		resources, err := transformer(cfg)
		require.NoError(t, err)

		testTransformFunc(t, findResource(t, resources, constants.LogVolumeID).TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
			require.NoError(t, err)

			assert.Equal(t, block.VolumeTypePartition, vc.TypedSpec().Type)
			assert.False(t, vc.TypedSpec().Mount.Secure, "promoted LOG should honor mount.secure=false")
		})
	})

	t.Run("custom diskSelector honored", func(t *testing.T) {
		t.Parallel()

		etcdCfg := blockcfg.NewVolumeConfigV1Alpha1()
		etcdCfg.MetaName = constants.EtcdDataVolumeID
		etcdCfg.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize("50GiB")
		require.NoError(t, etcdCfg.ProvisioningSpec.DiskSelectorSpec.Match.UnmarshalText([]byte(`disk.transport == "nvme"`)))

		cfg, err := container.New(baseCfg.DeepCopy(), etcdCfg)
		require.NoError(t, err)

		transformer := volumeconfig.GetPromotableSystemVolumesTransformer(false)
		resources, err := transformer(cfg)
		require.NoError(t, err)

		testTransformFunc(t, findResource(t, resources, constants.EtcdDataVolumeID).TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
			require.NoError(t, err)

			diskSelector, err := vc.TypedSpec().Provisioning.DiskSelector.Match.MarshalText()
			require.NoError(t, err)
			assert.Equal(t, `disk.transport == "nvme"`, string(diskSelector))
		})
	})

	t.Run("grow defaults to false", func(t *testing.T) {
		t.Parallel()

		transformer := volumeconfig.GetPromotableSystemVolumesTransformer(false)
		resources, err := transformer(promoteCfg(t, constants.EtcdDataVolumeID))
		require.NoError(t, err)

		testTransformFunc(t, findResource(t, resources, constants.EtcdDataVolumeID).TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
			require.NoError(t, err)

			// unlike EPHEMERAL (grow defaults to true), promoted system volumes default to grow=false.
			assert.False(t, vc.TypedSpec().Provisioning.PartitionSpec.Grow)
		})
	})

	t.Run("encryption wired", func(t *testing.T) {
		t.Parallel()

		etcdCfg := blockcfg.NewVolumeConfigV1Alpha1()
		etcdCfg.MetaName = constants.EtcdDataVolumeID
		etcdCfg.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize("50GiB")
		etcdCfg.EncryptionSpec.EncryptionProvider = block.EncryptionProviderLUKS2
		etcdCfg.EncryptionSpec.EncryptionKeys = []blockcfg.EncryptionKey{
			{
				KeySlot: 0,
				KeyStatic: &blockcfg.EncryptionKeyStatic{
					KeyData: "topsecret",
				},
			},
		}

		cfg, err := container.New(baseCfg.DeepCopy(), etcdCfg)
		require.NoError(t, err)

		transformer := volumeconfig.GetPromotableSystemVolumesTransformer(false)
		resources, err := transformer(cfg)
		require.NoError(t, err)

		testTransformFunc(t, findResource(t, resources, constants.EtcdDataVolumeID).TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
			require.NoError(t, err)

			assert.Equal(t, block.VolumeTypePartition, vc.TypedSpec().Type)
			assert.NotEmpty(t, vc.TypedSpec().Encryption, "configured encryption must be wired into the promoted volume")
			assert.Equal(t, block.EncryptionProviderLUKS2, vc.TypedSpec().Encryption.Provider)
		})
	})

	t.Run("always directory in container", func(t *testing.T) {
		t.Parallel()

		etcdCfg := blockcfg.NewVolumeConfigV1Alpha1()
		etcdCfg.MetaName = constants.EtcdDataVolumeID
		etcdCfg.ProvisioningSpec.ProvisioningMaxSize = blockcfg.MustSize("50GiB")

		cfg, err := container.New(baseCfg.DeepCopy(), etcdCfg)
		require.NoError(t, err)

		transformer := volumeconfig.GetPromotableSystemVolumesTransformer(true)
		resources, err := transformer(cfg)
		require.NoError(t, err)

		testTransformFunc(t, findResource(t, resources, constants.EtcdDataVolumeID).TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
			require.NoError(t, err)

			assert.Equal(t, block.VolumeTypeDirectory, vc.TypedSpec().Type)
		})
	})
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

func TestEphemeralVolumeMinAllocationGroupSize(t *testing.T) {
	t.Parallel()

	const defaultMinAGSize = 64 * 1024 * 1024 * 1024

	t.Run("Talos default without configuration", func(t *testing.T) {
		t.Parallel()

		transformer := volumeconfig.GetEphemeralVolumeTransformer(false)
		resources, err := transformer(container.NewV1Alpha1(&baseCfg))
		require.NoError(t, err)
		require.Len(t, resources, 1)

		testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
			require.NoError(t, err)
			assert.EqualValues(t, defaultMinAGSize, vc.TypedSpec().Provisioning.FilesystemSpec.MinAllocationGroupSize)
		})
	})

	t.Run("overridden by VolumeConfig", func(t *testing.T) {
		t.Parallel()

		ephemeralCfg := blockcfg.NewVolumeConfigV1Alpha1()
		ephemeralCfg.MetaName = constants.EphemeralPartitionLabel
		ephemeralCfg.FilesystemSpec.XFSSpec = &blockcfg.XFSSpec{
			MinAllocationGroupSizeConfig: blockcfg.MustByteSize("16GiB"),
		}

		cfg, err := container.New(baseCfg.DeepCopy(), ephemeralCfg)
		require.NoError(t, err)

		transformer := volumeconfig.GetEphemeralVolumeTransformer(false)
		resources, err := transformer(cfg)
		require.NoError(t, err)
		require.Len(t, resources, 1)

		testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
			require.NoError(t, err)
			assert.EqualValues(t, 16*1024*1024*1024, vc.TypedSpec().Provisioning.FilesystemSpec.MinAllocationGroupSize)
		})
	})

	t.Run("mkfs defaults when set to zero", func(t *testing.T) {
		t.Parallel()

		ephemeralCfg := blockcfg.NewVolumeConfigV1Alpha1()
		ephemeralCfg.MetaName = constants.EphemeralPartitionLabel
		ephemeralCfg.FilesystemSpec.XFSSpec = &blockcfg.XFSSpec{
			MinAllocationGroupSizeConfig: blockcfg.MustByteSize("0"),
		}

		cfg, err := container.New(baseCfg.DeepCopy(), ephemeralCfg)
		require.NoError(t, err)

		transformer := volumeconfig.GetEphemeralVolumeTransformer(false)
		resources, err := transformer(cfg)
		require.NoError(t, err)
		require.Len(t, resources, 1)

		testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
			require.NoError(t, err)
			assert.Zero(t, vc.TypedSpec().Provisioning.FilesystemSpec.MinAllocationGroupSize)
		})
	})
}

func TestEphemeralVolumeSecure(t *testing.T) {
	t.Parallel()

	t.Run("default is not secure", func(t *testing.T) {
		t.Parallel()

		transformer := volumeconfig.GetEphemeralVolumeTransformer(false)
		resources, err := transformer(container.NewV1Alpha1(&baseCfg))
		require.NoError(t, err)
		require.Len(t, resources, 1)

		testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
			require.NoError(t, err)
			assert.False(t, vc.TypedSpec().Mount.Secure, "EPHEMERAL should not be secure without explicit configuration")
		})
	})

	t.Run("VolumeConfig defaults to secure", func(t *testing.T) {
		t.Parallel()

		ephemeralCfg := blockcfg.NewVolumeConfigV1Alpha1()
		ephemeralCfg.MetaName = constants.EphemeralPartitionLabel

		cfg, err := container.New(baseCfg.DeepCopy(), ephemeralCfg)
		require.NoError(t, err)

		transformer := volumeconfig.GetEphemeralVolumeTransformer(false)
		resources, err := transformer(cfg)
		require.NoError(t, err)
		require.Len(t, resources, 1)

		testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
			require.NoError(t, err)
			assert.True(t, vc.TypedSpec().Mount.Secure)
		})
	})

	for _, secure := range []bool{false, true} {
		t.Run(fmt.Sprintf("secure=%t via VolumeConfig", secure), func(t *testing.T) {
			t.Parallel()

			ephemeralCfg := blockcfg.NewVolumeConfigV1Alpha1()
			ephemeralCfg.MetaName = constants.EphemeralPartitionLabel
			ephemeralCfg.MountSpec.MountSecure = &secure

			cfg, err := container.New(baseCfg.DeepCopy(), ephemeralCfg)
			require.NoError(t, err)

			transformer := volumeconfig.GetEphemeralVolumeTransformer(false)
			resources, err := transformer(cfg)
			require.NoError(t, err)
			require.Len(t, resources, 1)

			testTransformFunc(t, resources[0].TransformFunc, func(t *testing.T, vc *block.VolumeConfig, err error) {
				require.NoError(t, err)
				assert.Equal(t, secure, vc.TypedSpec().Mount.Secure)
			})
		})
	}
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
