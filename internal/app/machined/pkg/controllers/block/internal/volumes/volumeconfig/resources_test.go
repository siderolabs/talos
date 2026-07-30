// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumeconfig_test

import (
	"net/url"
	"slices"
	"testing"

	"github.com/siderolabs/gen/value"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/volumes"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/volumes/volumeconfig"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// TestBuildVolumeResourcesProvisioningOrder verifies that BuildVolumeResources returns volume configs
// in the very order VolumeManagerController provisions them.
//
// The manager picks up volume configs as they are published, so it may observe any prefix of the
// published sequence. Publishing out of provisioning order lets it provision a volume too early:
// notably, EPHEMERAL grows to fill the system disk, so provisioning it before the promotable system
// volumes which share the disk leaves them with no space at all.
func TestBuildVolumeResourcesProvisioningOrder(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		promoteSystemVolumes bool

		// expected order of the volumes which get a partition provisioned on the system disk
		expectedPartitions []string
	}{
		{
			// the promotable system volumes are directories under EPHEMERAL, so they don't get a
			// partition at all: STATE is provisioned first (it is the smallest), and EPHEMERAL last
			// as it grows to fill whatever is left.
			name: "default layout",

			expectedPartitions: []string{
				constants.StatePartitionLabel,
				constants.EphemeralPartitionLabel,
			},
		},
		{
			// each promotable system volume is capped at 50GiB and doesn't grow, so all of them have
			// to be provisioned before the growing EPHEMERAL.
			name: "promoted system volumes",

			promoteSystemVolumes: true,

			expectedPartitions: []string{
				constants.StatePartitionLabel,
				constants.CRIContainerdVolumeID,
				constants.EtcdDataVolumeID,
				constants.KubeletDataVolumeID,
				constants.LogVolumeID,
				constants.EphemeralPartitionLabel,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			resources, err := volumeconfig.BuildVolumeResources(
				t.Context(), machineConfig(t, test.promoteSystemVolumes), nil,
				machineruntime.ModeMetal.InContainer(), machineruntime.ModeMetal.IsAgent(),
			)
			require.NoError(t, err)

			configs := transformAll(t, resources)

			// the published order is the order the manager provisions volumes in
			assert.True(t, slices.IsSortedFunc(configs, volumes.CompareVolumeConfigs),
				"volume configs are not published in provisioning order: %v", volumeIDs(configs))

			partitions := xslices.Filter(configs, func(c *block.VolumeConfig) bool {
				return c.TypedSpec().Type == block.VolumeTypePartition && !value.IsZero(c.TypedSpec().Provisioning)
			})

			assert.Equal(t, test.expectedPartitions, volumeIDs(partitions))

			// Directory-backed volumes have no provisioning instructions at all, so they are published
			// before EPHEMERAL rather than after it. That is fine (and is what the manager has always
			// done with them): the manager marks a directory volume ready without touching the disk,
			// and MountController only creates the directory once the parent mount is up.
			ephemeralIdx := slices.Index(volumeIDs(configs), constants.EphemeralPartitionLabel)

			for _, c := range configs {
				if c.TypedSpec().Type != block.VolumeTypeDirectory {
					continue
				}

				assert.Less(t, slices.Index(volumeIDs(configs), c.Metadata().ID()), ephemeralIdx,
					"directory volume %q should be published before EPHEMERAL", c.Metadata().ID())
			}
		})
	}
}

// transformAll builds the VolumeConfig each resource would be published as, preserving the order.
func transformAll(t *testing.T, resources []volumeconfig.VolumeResource) []*block.VolumeConfig {
	t.Helper()

	return xslices.Map(resources, func(r volumeconfig.VolumeResource) *block.VolumeConfig {
		volumeConfig := block.NewVolumeConfig(block.NamespaceName, r.VolumeID)

		require.NoError(t, r.TransformFunc(volumeConfig))

		return volumeConfig
	})
}

func volumeIDs(configs []*block.VolumeConfig) []string {
	return xslices.Map(configs, func(c *block.VolumeConfig) string { return c.Metadata().ID() })
}

// machineConfig returns a minimal machine config, optionally promoting every promotable system volume
// onto a dedicated partition on the system disk, next to a growing EPHEMERAL.
func machineConfig(t *testing.T, promoteSystemVolumes bool) configconfig.Config {
	t.Helper()

	u, err := url.Parse("https://foo:6443")
	require.NoError(t, err)

	docs := []configconfig.Document{
		&v1alpha1.Config{
			ConfigVersion: "v1alpha1",
			MachineConfig: &v1alpha1.MachineConfig{},
			ClusterConfig: &v1alpha1.ClusterConfig{
				ControlPlane: &v1alpha1.ControlPlaneConfig{
					Endpoint: &v1alpha1.Endpoint{
						URL: u,
					},
				},
			},
		},
	}

	if promoteSystemVolumes {
		for _, volumeID := range configconfig.PromotableSystemVolumeNames {
			docs = append(docs, &blockcfg.VolumeConfigV1Alpha1{
				MetaName:         volumeID,
				ProvisioningSpec: blockcfg.ProvisioningSpec{ProvisioningMaxSize: blockcfg.MustSize("50GiB")},
			})
		}
	}

	ctr, err := container.New(docs...)
	require.NoError(t, err)

	return ctr
}
