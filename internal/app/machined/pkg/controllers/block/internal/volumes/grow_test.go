// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumes_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/siderolabs/gen/xerrors"
	"github.com/siderolabs/go-blockdevice/v2/partitioning/gpt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/volumes"
	blockpb "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/block"
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

			expectedSize: 1<<23 - 2*(1<<20),
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

			expectedSize: 1 << 22,
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

			require.NoError(t, retryRetryable(func() error { return volumes.Grow(ctx, logger, managerContext) }))

			assert.Equal(t, block.VolumePhaseProvisioned, volumeStatus.Phase)
			assert.Equal(t, test.expectedSize, volumeStatus.Size)
		})
	}
}

func TestGrowLive(t *testing.T) {
	checkRequirements(t)

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	t.Cleanup(cancel)

	logger := zaptest.NewLogger(t)

	rawImage := filepath.Join(t.TempDir(), "image.raw")

	f, err := os.Create(rawImage)
	require.NoError(t, err)

	require.NoError(t, f.Truncate(1<<23))
	require.NoError(t, f.Close())

	loDev := losetupAttachHelper(t, rawImage, false)

	t.Cleanup(func() {
		assert.NoError(t, loDev.Detach())
	})

	diskPath := loDev.Path()

	prepareGPT(
		t, diskPath,
		func(pt *gpt.Table) {
			_, _, err := pt.AllocatePartition(1<<20, "GROWS", uuid.MustParse("c12a7328-f81f-11d2-ba4b-00a0c93ec93b"))
			require.NoError(t, err)
		},
	)

	volumeCfg := block.NewVolumeConfig(block.NamespaceName, "TEST")
	*volumeCfg.TypedSpec() = block.VolumeConfigSpec{
		Type: block.VolumeTypePartition,
		Provisioning: block.ProvisioningSpec{
			PartitionSpec: block.PartitionSpec{
				MinSize: 1 << 20,
				Grow:    true,
			},
		},
	}

	volumeStatus := block.VolumeStatusSpec{
		Size:           1 << 20,
		PartitionIndex: 1,
		ParentLocation: diskPath,
	}

	disk := &blockpb.DiskSpec{
		DevPath: diskPath,
		Size:    1 << 23,
	}

	managerContext := volumes.ManagerContext{
		Cfg:               volumeCfg,
		Status:            &volumeStatus,
		Disks:             []volumes.DiskContext{{Disk: disk}},
		ObservedDiskSizes: map[string]uint64{},
	}

	// initial growth on provisioning
	require.NoError(t, retryRetryable(func() error { return volumes.Grow(ctx, logger, managerContext) }))

	assert.Equal(t, block.VolumePhaseProvisioned, volumeStatus.Phase)
	assert.Equal(t, uint64(1<<23-2*(1<<20)), volumeStatus.Size)
	assert.Equal(t, uint64(1<<23), managerContext.ObservedDiskSizes["TEST"])

	// disk size hasn't changed, nothing to do
	require.NoError(t, retryRetryable(func() error { return volumes.GrowLive(ctx, logger, managerContext) }))

	assert.Equal(t, uint64(1<<23-2*(1<<20)), volumeStatus.Size)

	// grow the disk
	require.NoError(t, os.Truncate(rawImage, 1<<24))

	loFile, err := os.Open(diskPath)
	require.NoError(t, err)

	require.NoError(t, unix.IoctlSetInt(int(loFile.Fd()), unix.LOOP_SET_CAPACITY, 0))
	require.NoError(t, loFile.Close())

	disk.Size = 1 << 24

	require.NoError(t, retryRetryable(func() error { return volumes.GrowLive(ctx, logger, managerContext) }))

	assert.Equal(t, uint64(1<<24-2*(1<<20)), volumeStatus.Size)
	assert.Equal(t, uint64(1<<24), managerContext.ObservedDiskSizes["TEST"])
}

func retryRetryable(f func() error) error {
	var err error

	for range 10 {
		err = f()
		if err != nil && xerrors.TagIs[volumes.Retryable](err) {
			// retry various disk locked and other retryable errors
			time.Sleep(10 * time.Millisecond)

			continue
		}

		break
	}

	return err
}
