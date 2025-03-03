// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumes_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/go-blockdevice/v2/partitioning/gpt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/volumes"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestCreatePartition(t *testing.T) {
	checkRequirements(t)

	for _, test := range []struct {
		name string

		diskSetup    func(t *testing.T) string
		volumeConfig block.VolumeConfigSpec
		hasPT        bool

		expectedPartitionIdx int
		expectedSize         uint64
	}{
		{
			name: "empty disk, fixed partition size",

			diskSetup: func(t *testing.T) string {
				return prepareRawImage(t, 1<<22)
			},
			volumeConfig: block.VolumeConfigSpec{
				Type: block.VolumeTypePartition,
				Provisioning: block.ProvisioningSpec{
					PartitionSpec: block.PartitionSpec{
						MinSize:  1 << 20,
						MaxSize:  1 << 20,
						Label:    "TEST1",
						TypeUUID: "c12a7328-f81f-11d2-ba4b-00a0c93ec93b",
					},
				},
			},
			hasPT: false,

			expectedPartitionIdx: 1,
			expectedSize:         1 << 20,
		},
		{
			name: "empty disk, with max size",

			diskSetup: func(t *testing.T) string {
				return prepareRawImage(t, 1<<22)
			},
			volumeConfig: block.VolumeConfigSpec{
				Type: block.VolumeTypePartition,
				Provisioning: block.ProvisioningSpec{
					PartitionSpec: block.PartitionSpec{
						MinSize:  1 << 20,
						MaxSize:  1 << 21,
						Label:    "TEST2",
						TypeUUID: "c12a7328-f81f-11d2-ba4b-00a0c93ec93b",
					},
				},
			},
			hasPT: false,

			expectedPartitionIdx: 1,
			expectedSize:         1 << 21,
		},
		{
			name: "empty disk, no max size",

			diskSetup: func(t *testing.T) string {
				return prepareRawImage(t, 1<<23)
			},
			volumeConfig: block.VolumeConfigSpec{
				Type: block.VolumeTypePartition,
				Provisioning: block.ProvisioningSpec{
					PartitionSpec: block.PartitionSpec{
						MinSize:  1 << 20,
						Label:    "TEST3",
						TypeUUID: "c12a7328-f81f-11d2-ba4b-00a0c93ec93c",
					},
				},
			},
			hasPT: false,

			expectedPartitionIdx: 1,
			expectedSize:         1<<23 - 1<<21, // partition grows to max available size minus GPT overhead/alignment
		},
		{
			name: "empty GPT, fixed partition size",

			diskSetup: func(t *testing.T) string {
				diskPath := prepareRawImage(t, 1<<22)

				prepareGPT(t, diskPath)

				return diskPath
			},
			volumeConfig: block.VolumeConfigSpec{
				Type: block.VolumeTypePartition,
				Provisioning: block.ProvisioningSpec{
					PartitionSpec: block.PartitionSpec{
						MinSize:  1 << 20,
						MaxSize:  1 << 20,
						Label:    "TEST4",
						TypeUUID: "c12a7328-f81f-11d2-ba4b-00a0c93ec93b",
					},
				},
			},
			hasPT: true,

			expectedPartitionIdx: 1,
			expectedSize:         1 << 20,
		},
		{
			name: "non-empty GPT, with max size",

			diskSetup: func(t *testing.T) string {
				diskPath := prepareRawImage(t, 1<<23)

				prepareGPT(t, diskPath,
					func(pt *gpt.Table) {
						_, _, err := pt.AllocatePartition(1<<20, "FIXED", uuid.MustParse("c12a7328-f81f-11d2-ba4b-00a0c93ec93b"))
						require.NoError(t, err)
					},
				)

				return diskPath
			},
			volumeConfig: block.VolumeConfigSpec{
				Type: block.VolumeTypePartition,
				Provisioning: block.ProvisioningSpec{
					PartitionSpec: block.PartitionSpec{
						MinSize:  1 << 20,
						MaxSize:  1 << 24,
						Label:    "TEST4",
						TypeUUID: "c12a7328-f81f-11d2-ba4b-00a0c93ec93b",
					},
				},
			},
			hasPT: true,

			expectedPartitionIdx: 2,
			expectedSize:         1<<23 - 3*(1<<20), // partition grows to max available size minus GPT overhead/alignment
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()

			diskPath := test.diskSetup(t)
			logger := zaptest.NewLogger(t)

			volumeCfg := block.NewVolumeConfig(block.NamespaceName, "TEST")
			*volumeCfg.TypedSpec() = test.volumeConfig

			var (
				result volumes.CreatePartitionResult
				err    error
			)

			for range 10 {
				result, err = volumes.CreatePartition(ctx, logger, diskPath, volumeCfg, test.hasPT)
				if err != nil && xerrors.TagIs[volumes.Retryable](err) {
					// retry various disk locked and other retryable errors
					time.Sleep(10 * time.Millisecond)

					continue
				}

				break
			}

			require.NoError(t, err)

			assert.Equal(t, test.expectedPartitionIdx, result.PartitionIdx)
			assert.Equal(t, test.expectedSize, result.Size)
		})
	}
}
