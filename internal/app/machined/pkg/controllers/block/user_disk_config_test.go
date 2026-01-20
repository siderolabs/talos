// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

type UserDiskConfigSuite struct {
	ctest.DefaultSuite
}

func TestUserDiskConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &UserDiskConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 3 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.UserDiskConfigController{}))
			},
		},
	})
}

func (suite *UserDiskConfigSuite) TestReconcileDefaults() {
	ctest.AssertNoResource[*block.UserDiskConfigStatus](suite, block.UserDiskConfigStatusID)

	// create a dummy machine config
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
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
		),
	)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	// now the volume config should be created
	ctest.AssertResource(suite, block.UserDiskConfigStatusID, func(r *block.UserDiskConfigStatus, asrt *assert.Assertions) {
		asrt.True(r.TypedSpec().Ready)
	})
}

func (suite *UserDiskConfigSuite) TestReconcileUserDisk() {
	ctest.AssertNoResource[*block.UserDiskConfigStatus](suite, block.UserDiskConfigStatusID)

	dir := suite.T().TempDir()

	disk1, disk2 := filepath.Join(dir, "disk1"), filepath.Join(dir, "disk2")

	suite.Require().NoError(os.WriteFile(disk1, nil, 0o644))
	suite.Require().NoError(os.WriteFile(disk2, nil, 0o644))

	// create a machine config with user disks
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineDisks: []*v1alpha1.MachineDisk{
						{
							DeviceName: disk1,
							DiskPartitions: []*v1alpha1.DiskPartition{
								{
									DiskSize:       1024 * 1024,
									DiskMountPoint: "/var/1-1",
								},
								{
									DiskSize:       1024 * 1024,
									DiskMountPoint: "/var/1-2",
								},
							},
						},
						{
							DeviceName: disk2,
							DiskPartitions: []*v1alpha1.DiskPartition{
								{
									DiskSize:       1024 * 1024,
									DiskMountPoint: "/var/2-1",
								},
							},
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
				},
			},
		),
	)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	// now the volume config should be created
	for _, id := range []string{disk1 + "-1", disk1 + "-2", disk2 + "-1"} {
		ctest.AssertResource(suite, id, func(r *block.VolumeConfig, asrt *assert.Assertions) {
			asrt.NotEmpty(r.TypedSpec().Provisioning)
			asrt.Contains(r.Metadata().Labels().Raw(), block.UserDiskLabel)
			asrt.GreaterOrEqual(r.TypedSpec().Provisioning.Wave, block.WaveLegacyUserDisks)
			asrt.Equal(constants.EphemeralPartitionLabel, r.TypedSpec().Mount.ParentID)
			asrt.NotContains(r.TypedSpec().Mount.TargetPath, "/") // path should become relative
		})
	}

	// .. and a volume mount request
	for _, id := range []string{disk1 + "-1", disk1 + "-2", disk2 + "-1"} {
		ctest.AssertResource(suite, id, func(r *block.VolumeMountRequest, asrt *assert.Assertions) {
			asrt.Equal(id, r.TypedSpec().VolumeID)
		})
	}

	// the status should not be ready (yet)
	ctest.AssertResource(suite, block.UserDiskConfigStatusID, func(r *block.UserDiskConfigStatus, asrt *assert.Assertions) {
		asrt.False(r.TypedSpec().Ready)
	})

	// now emulate that the mount requests are fulfilled
	for _, id := range []string{disk1 + "-1", disk1 + "-2", disk2 + "-1"} {
		volumeMountStatus := block.NewVolumeMountStatus(id)
		suite.Create(volumeMountStatus)

		suite.AddFinalizer(block.NewVolumeMountRequest(block.NamespaceName, id).Metadata(), "test")
	}

	// the controller should put a finalizer on the mount status
	for _, id := range []string{disk1 + "-1", disk1 + "-2", disk2 + "-1"} {
		ctest.AssertResource(suite, id, func(r *block.VolumeMountStatus, asrt *assert.Assertions) {
			asrt.True(r.Metadata().Finalizers().Has((&blockctrls.UserDiskConfigController{}).Name()))
		})
	}

	// now everything should be ready
	ctest.AssertResource(suite, block.UserDiskConfigStatusID, func(r *block.UserDiskConfigStatus, asrt *assert.Assertions) {
		asrt.True(r.TypedSpec().Ready)
		asrt.False(r.TypedSpec().TornDown)
	})

	// start tearing down volume mount status
	for _, id := range []string{disk1 + "-1", disk1 + "-2", disk2 + "-1"} {
		_, err := suite.State().Teardown(suite.Ctx(), block.NewVolumeMountStatus(id).Metadata())
		suite.Require().NoError(err)
	}

	// back to not ready
	ctest.AssertResource(suite, block.UserDiskConfigStatusID, func(r *block.UserDiskConfigStatus, asrt *assert.Assertions) {
		asrt.False(r.TypedSpec().Ready)
		asrt.False(r.TypedSpec().TornDown)
	})

	// the finalizers on mount statuses should be removed
	for _, id := range []string{disk1 + "-1", disk1 + "-2", disk2 + "-1"} {
		ctest.AssertResource(suite, id, func(r *block.VolumeMountStatus, asrt *assert.Assertions) {
			asrt.True(r.Metadata().Finalizers().Empty())
		})
	}

	// remove the finalizer from the mount request
	for _, id := range []string{disk1 + "-1", disk1 + "-2", disk2 + "-1"} {
		suite.RemoveFinalizer(block.NewVolumeMountRequest(block.NamespaceName, id).Metadata(), "test")
	}
}
