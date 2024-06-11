// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/volumes"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestCheckDiskForProvisioning(t *testing.T) {
	checkRequirements(t)

	for _, test := range []struct {
		name string

		diskSetup    func(t *testing.T) string
		volumeConfig block.VolumeConfigSpec

		expected volumes.CheckDiskResult
	}{
		{
			name: "small empty disk",

			diskSetup: func(t *testing.T) string {
				return prepareRawImage(t, 1<<20)
			},
			volumeConfig: block.VolumeConfigSpec{
				Type: block.VolumeTypePartition,
				Provisioning: block.ProvisioningSpec{
					PartitionSpec: block.PartitionSpec{
						MinSize: 1 << 20,
					},
				},
			},

			expected: volumes.CheckDiskResult{
				DiskSize: 1 << 20,
			},
		},
		{
			name: "big enough empty disk",

			diskSetup: func(t *testing.T) string {
				return prepareRawImage(t, 1<<20)
			},
			volumeConfig: block.VolumeConfigSpec{
				Type: block.VolumeTypePartition,
				Provisioning: block.ProvisioningSpec{
					PartitionSpec: block.PartitionSpec{
						MinSize: 1 << 18,
					},
				},
			},

			expected: volumes.CheckDiskResult{
				CanProvision: true,
				DiskSize:     1 << 20,
			},
		},
		{
			name: "big enough formatted disk",

			diskSetup: func(t *testing.T) string {
				disk := prepareRawImage(t, 1<<21)

				formatExt4(t, disk)

				return disk
			},
			volumeConfig: block.VolumeConfigSpec{
				Type: block.VolumeTypePartition,
				Provisioning: block.ProvisioningSpec{
					PartitionSpec: block.PartitionSpec{
						MinSize: 1 << 18,
					},
				},
			},

			expected: volumes.CheckDiskResult{
				CanProvision: false,
			},
		},
		{
			name: "big enough empty GPT disk",

			diskSetup: func(t *testing.T) string {
				disk := prepareRawImage(t, 1<<24)

				prepareGPT(t, disk)

				return disk
			},
			volumeConfig: block.VolumeConfigSpec{
				Type: block.VolumeTypePartition,
				Provisioning: block.ProvisioningSpec{
					PartitionSpec: block.PartitionSpec{
						MinSize: 1 << 20,
					},
				},
			},

			expected: volumes.CheckDiskResult{
				CanProvision: true,
				HasGPT:       true,
				DiskSize:     1 << 24,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			diskPath := test.diskSetup(t)
			logger := zaptest.NewLogger(t)

			volumeCfg := block.NewVolumeConfig(block.NamespaceName, "TEST")
			*volumeCfg.TypedSpec() = test.volumeConfig

			assert.Equal(t, test.expected, volumes.CheckDiskForProvisioning(logger, diskPath, volumeCfg))
		})
	}
}
