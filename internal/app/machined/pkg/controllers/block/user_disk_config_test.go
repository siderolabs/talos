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
		})
	}

	ctest.AssertResource(suite, block.UserDiskConfigStatusID, func(r *block.UserDiskConfigStatus, asrt *assert.Assertions) {
		asrt.True(r.TypedSpec().Ready)
	})
}
