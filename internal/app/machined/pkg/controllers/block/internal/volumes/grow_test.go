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

		expectedGrew    bool
		expectedNewSize uint64
	}{
		{
			name: "grow at the end of the disk",

			diskSetup: func(t *testing.T) string {
				diskPath := prepareRawImage(t, 1<<23)

				prepareGPT(
					t, diskPath,
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

			expectedGrew:    true,
			expectedNewSize: 1<<23 - 2*(1<<20),
		},
		{
			name: "grow to max size",

			diskSetup: func(t *testing.T) string {
				diskPath := prepareRawImage(t, 1<<23)

				prepareGPT(
					t, diskPath,
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

			expectedGrew:    true,
			expectedNewSize: 1 << 22,
		},
		{
			name: "doesn't grow at max size",

			diskSetup: func(t *testing.T) string {
				diskPath := prepareRawImage(t, 1<<23)

				prepareGPT(
					t, diskPath,
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

			expectedGrew:    false,
			expectedNewSize: 0,
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

			originalSize := volumeStatus.Size

			managerContext := volumes.ManagerContext{
				Cfg:    volumeCfg,
				Status: &volumeStatus,
			}

			var (
				grew    bool
				newSize uint64
				err     error
			)

			for range 10 {
				grew, newSize, err = volumes.Grow(ctx, logger, managerContext)
				if err != nil && xerrors.TagIs[volumes.Retryable](err) {
					// retry various disk locked and other retryable errors
					time.Sleep(10 * time.Millisecond)

					continue
				}

				break
			}

			require.NoError(t, err)

			assert.Equal(t, test.expectedGrew, grew, "unexpected grew value")
			assert.Equal(t, test.expectedNewSize, newSize, "unexpected newSize value")

			// Grow() must not modify Phase or Size; those are the caller's responsibility.
			assert.Equal(t, block.VolumePhaseWaiting, volumeStatus.Phase, "Grow() must not modify Phase")
			assert.Equal(t, originalSize, volumeStatus.Size, "Grow() must not modify Status.Size")
		})
	}
}
