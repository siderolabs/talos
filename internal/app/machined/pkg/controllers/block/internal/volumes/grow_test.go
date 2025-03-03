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

//nolint:dupl
func TestGrow(t *testing.T) {
	checkRequirements(t)

	for _, test := range []struct {
		name string

		diskSetup    func(t *testing.T) string
		volumeConfig block.VolumeConfigSpec
		volumeStatus block.VolumeStatusSpec

		expectedSize uint64
	}{
		{
			name: "grow at the end of the disk",

			diskSetup: func(t *testing.T) string {
				diskPath := prepareRawImage(t, 1<<23)

				prepareGPT(t, diskPath,
					func(pt *gpt.Table) {
						_, _, err := pt.AllocatePartition(1<<20, "GROWS", uuid.MustParse("c12a7328-f81f-11d2-ba4b-00a0c93ec93b"))
						require.NoError(t, err)
					},
				)

				return diskPath
			},
			volumeConfig: block.VolumeConfigSpec{
				Type: block.VolumeTypePartition,
				Provisioning: block.ProvisioningSpec{
					PartitionSpec: block.PartitionSpec{
						MinSize: 1 << 20,
						Grow:    true,
					},
				},
			},
			volumeStatus: block.VolumeStatusSpec{
				Size:           1 << 20,
				PartitionIndex: 1,
			},

			expectedSize: 1<<23 - 2*(1<<20),
		},
		{
			name: "grow to max size",

			diskSetup: func(t *testing.T) string {
				diskPath := prepareRawImage(t, 1<<23)

				prepareGPT(t, diskPath,
					func(pt *gpt.Table) {
						_, _, err := pt.AllocatePartition(1<<20, "GROWS", uuid.MustParse("c12a7328-f81f-11d2-ba4b-00a0c93ec93b"))
						require.NoError(t, err)
					},
				)

				return diskPath
			},
			volumeConfig: block.VolumeConfigSpec{
				Type: block.VolumeTypePartition,
				Provisioning: block.ProvisioningSpec{
					PartitionSpec: block.PartitionSpec{
						MinSize: 1 << 20,
						MaxSize: 1 << 22,
						Grow:    true,
					},
				},
			},
			volumeStatus: block.VolumeStatusSpec{
				Size:           1 << 20,
				PartitionIndex: 1,
			},

			expectedSize: 1 << 22,
		},
		{
			name: "doesn't grow at max size",

			diskSetup: func(t *testing.T) string {
				diskPath := prepareRawImage(t, 1<<23)

				prepareGPT(t, diskPath,
					func(pt *gpt.Table) {
						_, _, err := pt.AllocatePartition(1<<21, "BIG", uuid.MustParse("c12a7328-f81f-11d2-ba4b-00a0c93ec93b"))
						require.NoError(t, err)
					},
				)

				return diskPath
			},
			volumeConfig: block.VolumeConfigSpec{
				Type: block.VolumeTypePartition,
				Provisioning: block.ProvisioningSpec{
					PartitionSpec: block.PartitionSpec{
						MinSize: 1 << 20,
						MaxSize: 1 << 21,
						Grow:    true,
					},
				},
			},
			volumeStatus: block.VolumeStatusSpec{
				Size:           1 << 21,
				PartitionIndex: 1,
			},

			expectedSize: 1 << 21,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			t.Cleanup(cancel)

			logger := zaptest.NewLogger(t)

			volumeCfg := block.NewVolumeConfig(block.NamespaceName, "TEST")
			*volumeCfg.TypedSpec() = test.volumeConfig

			volumeStatus := test.volumeStatus
			volumeStatus.ParentLocation = test.diskSetup(t)

			managerContext := volumes.ManagerContext{
				Cfg:    volumeCfg,
				Status: &volumeStatus,
			}

			var err error

			for range 10 {
				err = volumes.Grow(ctx, logger, managerContext)
				if err != nil && xerrors.TagIs[volumes.Retryable](err) {
					// retry various disk locked and other retryable errors
					time.Sleep(10 * time.Millisecond)

					continue
				}

				break
			}

			require.NoError(t, err)

			assert.Equal(t, block.VolumePhaseProvisioned, volumeStatus.Phase)
			assert.Equal(t, test.expectedSize, volumeStatus.Size)
		})
	}
}
