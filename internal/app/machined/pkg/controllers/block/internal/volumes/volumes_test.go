// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/volumes"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestCompareVolumeConfigs(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		a *block.VolumeConfigSpec
		b *block.VolumeConfigSpec

		expected int
	}{
		{
			name: "different wave",

			a: &block.VolumeConfigSpec{
				Provisioning: block.ProvisioningSpec{
					Wave: block.WaveSystemDisk,
				},
			},
			b: &block.VolumeConfigSpec{
				Provisioning: block.ProvisioningSpec{
					Wave: block.WaveUserVolumes,
				},
			},

			expected: -1,
		},
		{
			name: "prefer grow",

			a: &block.VolumeConfigSpec{
				Provisioning: block.ProvisioningSpec{
					Wave: block.WaveSystemDisk,
					PartitionSpec: block.PartitionSpec{
						Grow: true,
					},
				},
			},
			b: &block.VolumeConfigSpec{
				Provisioning: block.ProvisioningSpec{
					Wave: block.WaveSystemDisk,
					PartitionSpec: block.PartitionSpec{
						Grow: false,
					},
				},
			},

			expected: 1,
		},
		{
			name: "prefer smaller size",

			a: &block.VolumeConfigSpec{
				Provisioning: block.ProvisioningSpec{
					Wave: block.WaveSystemDisk,
					PartitionSpec: block.PartitionSpec{
						Grow:    false,
						MinSize: 100,
						MaxSize: 200,
					},
				},
			},
			b: &block.VolumeConfigSpec{
				Provisioning: block.ProvisioningSpec{
					Wave: block.WaveSystemDisk,
					PartitionSpec: block.PartitionSpec{
						Grow:    false,
						MinSize: 150,
						MaxSize: 1000,
					},
				},
			},

			expected: -1,
		},
		{
			name: "prefer max size",

			a: &block.VolumeConfigSpec{
				Provisioning: block.ProvisioningSpec{
					Wave: block.WaveSystemDisk,
					PartitionSpec: block.PartitionSpec{
						Grow:    false,
						MinSize: 100,
						MaxSize: 200,
					},
				},
			},
			b: &block.VolumeConfigSpec{
				Provisioning: block.ProvisioningSpec{
					Wave: block.WaveSystemDisk,
					PartitionSpec: block.PartitionSpec{
						Grow:    false,
						MinSize: 50,
						MaxSize: 0, // no limit
					},
				},
			},

			expected: -1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			resA := block.NewVolumeConfig(block.NamespaceName, "A")
			*resA.TypedSpec() = *test.a

			resB := block.NewVolumeConfig(block.NamespaceName, "B")
			*resB.TypedSpec() = *test.b

			actual := volumes.CompareVolumeConfigs(resA, resB)

			assert.Equal(t, test.expected, actual)
		})
	}
}
